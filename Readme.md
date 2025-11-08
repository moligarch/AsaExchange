# AsaExchange Bot
AsaExchange is a high-performance, event-driven Telegram bot monolith designed to facilitate peer-to-peer (P2P) currency exchanges. This system acts as a marketplace and a secure, admin-verified platform, built using a modular, decoupled architecture in Go.
## Current Status: Phase 1 Complete
This repository contains the complete foundation for the application. The **User Registration and Admin Verification** flow is fully implemented and end-to-end testable.

A new user can:
1. Start the bot and be guided through a stateful registration process.
2. Submit their First Name, Last Name, and Phone Number (via contact sharing).
3. Submit their Government ID (text) and select their Country from a list.
4. Upload a photo of their ID document.
5. Accept or Decline the service's policy.

An admin can:
1. Be automatically notified of new pending users in a private admin channel.
2. See the user's photo and all submitted data in a single message.
3. Approve or Reject the user with a single button click.
4. The user is then automatically notified of their new status via the Customer Bot.

## Core Architecture
This project is a **Modular Monolith** built on **Hexagonal (Ports & Adapters)** principles and a sophisticated **Event-Driven** design.
1. **In-Process Event Bus**: The core of the monolith. A central `EventBus` (`adapters/eventbus`) decouples all major components. For example, the `ModeratorServer` (which polls Telegram) simply publishes raw updates to the bus. The `ModeratorRouter` subscribes to these events to handle commands, ensuring the poller and the processor are separate.
2. **Dual Bot System**: The application runs two bots from a single binary:
 - **Customer Bot** (`bot/customer`): Handles all user-facing interactions (registration, and in the future, requests/bids).
 - **Moderator Bot** (`bot/moderator`): Handles all secure admin/system tasks (user verification, and in the future, transaction management).
3. **"Channel-as-Queue"** (`VerificationQueue`): We use a robust, decoupled queue for handling new user verifications.
 - **Interface**: `ports.VerificationQueue` defines a Publish and Subscribe contract.
 - **MVP** Implementation: `adapters/telegram/queue.go`.
   + `Publish`: The `registration_handler` (Customer Bot) publishes a new user by sending their photo and data to a private "Upload" channel. It saves the `message_id` as the storage reference.
   + `Subscribe`: The `Orchestrator` launches a dedicated listener (using the Moderator Bot's token) that polls this "Upload" channel. When a message arrives, it fires an event.
   + **Future-Proof**: This entire `TelegramQueue` adapter can be cleanly swapped with a `MinioRedisQueue` adapter in the future without changing a single line of business logic.
4. **Stateful Handlers**: User registration is managed by a Finite State Machine (FSM) in `bot/customer/handlers/registration_handler.go`. This `MessageHandler` routes incoming text/photo replies based on the `user_state` (e.g., `awaiting_first_name`, `awaiting_identity_doc`) stored in the database.
5. **Secure Persistence**: All database logic is handled by `adapters/postgres`.
 - Uses `pgx` for connection pooling.
 - Uses `golang-migrate` for schema versioning.
 - Implements a `SecurityPort` (`adapters/security`) to encrypt all PII (phone, Gov ID) using AES-GCM before it's saved in the database.
 
### Tech Stack
- **Core**:Go 1.21+
- **Database**: PostgreSQL 16
- **Migrations**: golang-migrate
- **DB Driver**: jackc/pgx
- **Telegram** API: go-telegram-bot-api/v5
- **Config**: viper (reading a single config.yaml)
- **Logging**: zerolog
- **Testing**: testify (for mocks and assertions)
- **Services**: docker-compose (for postgres)
 
## How to Run
1. **Configure**:
 - Copy config.example.yaml to config.yaml.
 - Fill in all secrets: encryption_key, postgres.url, bot.customer.token, bot.moderator.token.
 - Create three private Telegram channels/groups.
 - Add your Customer Bot as an admin (with "Post messages") to the "Upload Channel".
 - Add your Moderator Bot as an admin to all three channels (Upload, Review, Public).
 - Get the Channel IDs (e.g., -100...) and add them to config.yaml (private_upload_channel_id, admin_review_channel_id, public_channel_id).

2. **Start Services**:

```bash
# (From project root)
docker-compose up -d postgres
```

3. **Run Migrations**:
Make sure you have golang-migrate installed.
Run the migrations. Note: You must run all migrations up to the latest version.

```bash
migrate -path 'internal/adapters/postgres/migrations' -database 'postgres://asa_user:a_very_secure_password_p123!@localhost:5432/asa_db?sslmode=disable' up
```

4. **Run the Application**:
```bash
# (From project root)
go run ./cmd/server
```
The application will start, and both bots will begin polling for updates.

## 📅 Next Moves: Plan for Phase 2
We have successfully built the complete User Management system. The next phase is to build the Marketplace logic.

1. Create New Database Tables:
 - We need requests (the "sell" offer), bids (the "buy" offer), and transactions (the matched deal).
 - (Self-correction: We already created these in our initial migration `000001_initial_schema.up.sql`. We are ready to use them.)

2. Build Customer Marketplace Handlers (`bot/customer/handlers/`):

 - `NewRequest Handlers`:
   + Create a new command handler new_request_handler.go for /newrequest.
   + This will trigger a new FSM (state machine) inside registration_handler.go (or a  new marketplace_handler.go).
   + It will ask the user (who must be level_1 verified) a series of questions:
     1. "What currency do you want to sell?" (e.g., "EUR")
     2. "What currency do you want to receive?" (e.g., "Rial")
     3. "How much [EUR] are you selling?"
     4. "What exchange rate are you asking for?"
   + Upon completion, it saves a new record to the requests table.

 - Event Publication: After saving the request, the handler will publish a new event to the EventBus, e.g., bus.Publish(ctx, "request:created", newRequest).
 - ListRequests Handler: Create a new command handler list_requests_handler.go for /listrequests that shows a paginated list of open requests from the DB.
 - Bid Handler: Create a new callback handler bid_handler.go that handles callbacks from the /listrequests message (e.g., bid_REQUEST-ID-HERE).

3. Build Moderator Marketplace Handlers (`bot/moderator/handlers/`):

  - `RequestPublisher Handler`:
    + Create a new system handler request_publisher.go (like the notification_handler).
    + It will Subscribe to the request:created event on the EventBus.
    + When it receives an event, it will use the Moderator Bot Client to format and post the new request to the public_channel_id.

  - `TransactionMonitor Handler`:
    + We will need a new FSM for matched transactions where users upload their receipts. This will be the most complex part of Phase 2.
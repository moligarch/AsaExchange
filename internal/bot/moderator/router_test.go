package moderator

import (
	"AsaExchange/internal/core/domain"
	"AsaExchange/internal/core/ports"
	"context"
	"testing"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"
)

// --- Mocks ---

// MockUserRepository
type MockUserRepository struct {
	mock.Mock
}

func (m *MockUserRepository) Create(ctx context.Context, user *domain.User) error {
	args := m.Called(ctx, user)
	return args.Error(0)
}
func (m *MockUserRepository) GetByTelegramID(ctx context.Context, id int64) (*domain.User, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.User), args.Error(1)
}
func (m *MockUserRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.User), args.Error(1)
}
func (m *MockUserRepository) Update(ctx context.Context, user *domain.User) error {
	args := m.Called(ctx, user)
	return args.Error(0)
}
func (m *MockUserRepository) Delete(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}
func (m *MockUserRepository) GetNextPendingUser(ctx context.Context) (*domain.User, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.User), args.Error(1)
}

// MockCommandHandler
type MockCommandHandler struct {
	mock.Mock
}

func (m *MockCommandHandler) Command() string {
	args := m.Called()
	return args.String(0)
}
func (m *MockCommandHandler) Handle(ctx context.Context, update *ports.BotUpdate) error {
	args := m.Called()
	return args.Error(0)
}

// MockCallbackHandler
type MockCallbackHandler struct {
	mock.Mock
}

func (m *MockCallbackHandler) Prefix() string {
	args := m.Called()
	return args.String(0)
}
func (m *MockCallbackHandler) Handle(ctx context.Context, update *ports.BotUpdate, user *domain.User) error {
	args := m.Called(ctx, update, user)
	return args.Error(0)
}

// MockBotClient
type MockBotClient struct {
	mock.Mock
}

func (m *MockBotClient) SendMessage(ctx context.Context, params ports.SendMessageParams) (int, error) {
	args := m.Called(ctx, params)
	return args.Int(0), args.Error(1)
}
func (mm *MockBotClient) SetMenuCommands(ctx context.Context, chatID int64, isAdmin bool) error {
	args := mm.Called(ctx, chatID, isAdmin)
	return args.Error(0)
}
func (m *MockBotClient) EditMessageText(ctx context.Context, params ports.EditMessageParams) error {
	args := m.Called(ctx, params)
	return args.Error(0)
}
func (m *MockBotClient) EditMessageCaption(ctx context.Context, params ports.EditMessageCaptionParams) error {
	args := m.Called(ctx, params)
	return args.Error(0)
}
func (m *MockBotClient) AnswerCallbackQuery(ctx context.Context, params ports.AnswerCallbackParams) error {
	args := m.Called(ctx, params)
	return args.Error(0)
}
func (m *MockBotClient) SendPhoto(ctx context.Context, params ports.SendPhotoParams) (int, error) {
	args := m.Called(ctx, params)
	return args.Int(0), args.Error(1)
}

// MockEventBus
type MockEventBus struct {
	mock.Mock
	Handlers map[string]ports.EventHandler
}

func (m *MockEventBus) Publish(ctx context.Context, topic string, data interface{}) error {
	args := m.Called(ctx, topic, data)
	return args.Error(0)
}
func (m *MockEventBus) Subscribe(topic string, handler ports.EventHandler) {
	m.Called(topic, handler)
	if m.Handlers == nil {
		m.Handlers = make(map[string]ports.EventHandler)
	}
	m.Handlers[topic] = handler // Store the handler so we can call it
}

// --- Tests ---

func TestModeratorRouter_HandleUpdate_Command(t *testing.T) {
	// 1. Setup
	ctx := context.Background()
	nopLogger := zerolog.Nop()
	mockUserRepo := new(MockUserRepository)
	mockBotClient := new(MockBotClient)
	mockBus := new(MockEventBus)

	// Setup expectations for the bus subscriptions
	mockBus.On("Subscribe", "telegram:mod:message", mock.Anything)
	mockBus.On("Subscribe", "telegram:mod:callback_query", mock.Anything)

	router := NewModeratorRouter(mockUserRepo, mockBotClient, mockBus, &nopLogger)

	// Create and register a mock handler
	reviewHandler := new(MockCommandHandler)
	reviewHandler.On("Command").Return("review")
	reviewHandler.On("Handle").Return(nil).Once()
	router.RegisterCommandHandler(reviewHandler)

	// 2. Create a fake Admin User
	adminUser := &domain.User{ID: uuid.New(), IsModerator: true}

	// 3. Create a fake Telegram update
	fakeUpdate := tgbotapi.Update{
		UpdateID: 123,
		Message: &tgbotapi.Message{
			MessageID: 456,
			From:      &tgbotapi.User{ID: 789, UserName: "adminuser"},
			Chat:      &tgbotapi.Chat{ID: 1000},
			Text:      "/review",
			Entities: []tgbotapi.MessageEntity{
				{Type: "bot_command", Offset: 0, Length: 7},
			},
		},
	}

	// 4. Define Expectations
	mockUserRepo.On("GetByTelegramID", mock.Anything, int64(789)).Return(adminUser, nil).Once()

	// 5. Run the handler
	// We simulate the event bus calling the router's handler
	handler := mockBus.Handlers["telegram:mod:message"]
	err := handler(ctx, ports.Event{Topic: "telegram:mod:message", Data: fakeUpdate})
	if err != nil {
		t.Fatalf("Handler returned an error: %v", err)
	}

	// 6. Assert expectations
	mockBus.AssertCalled(t, "Subscribe", "telegram:mod:message", mock.Anything)
	mockUserRepo.AssertExpectations(t)
	reviewHandler.AssertExpectations(t)
}

func TestModeratorRouter_CallbackRouting(t *testing.T) {
	// 1. Setup
	ctx := context.Background()
	nopLogger := zerolog.Nop()
	mockUserRepo := new(MockUserRepository)
	mockBotClient := new(MockBotClient)
	mockBus := new(MockEventBus)

	// Setup expectations for the bus subscriptions
	mockBus.On("Subscribe", "telegram:mod:message", mock.Anything)
	mockBus.On("Subscribe", "telegram:mod:callback_query", mock.Anything)

	router := NewModeratorRouter(mockUserRepo, mockBotClient, mockBus, &nopLogger)

	// 2. Create a fake Admin User
	adminUser := &domain.User{ID: uuid.New(), IsModerator: true}

	// 3. Create and register a mock handler
	approvalHandler := new(MockCallbackHandler)
	approvalHandler.On("Prefix").Return("approval_")
	approvalHandler.On("Handle", mock.Anything, mock.Anything, adminUser).Return(nil).Once()
	router.RegisterCallbackHandler(approvalHandler)

	// 4. Create a fake Telegram update
	fakeUpdate := tgbotapi.Update{
		UpdateID: 124,
		CallbackQuery: &tgbotapi.CallbackQuery{
			ID:   "cb_id_1",
			From: &tgbotapi.User{ID: 789, UserName: "adminuser"},
			Message: &tgbotapi.Message{
				MessageID: 456,
				Chat:      &tgbotapi.Chat{ID: 1000},
			},
			Data: "approval_accept_...",
		},
	}

	// 5. Define Expectations
	mockUserRepo.On("GetByTelegramID", mock.Anything, int64(789)).Return(adminUser, nil).Once()

	// 6. Run the handler
	handler := mockBus.Handlers["telegram:mod:callback_query"]
	err := handler(ctx, ports.Event{Topic: "telegram:mod:callback_query", Data: fakeUpdate})
	if err != nil {
		t.Fatalf("Handler returned an error: %v", err)
	}

	// 7. Assert expectations
	mockBus.AssertCalled(t, "Subscribe", "telegram:mod:callback_query", mock.Anything)
	mockUserRepo.AssertExpectations(t)
	approvalHandler.AssertExpectations(t)
}

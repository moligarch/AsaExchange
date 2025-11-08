package customer

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

var _ ports.UserRepository = (*MockUserRepository)(nil)

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

// MockBotClient is a mock for the BotClientPort
type MockBotClient struct {
	mock.Mock
}

var _ ports.BotClientPort = (*MockBotClient)(nil)

func (m *MockBotClient) SendMessage(ctx context.Context, params ports.SendMessageParams) (int, error) {
	args := m.Called(ctx, params)
	return args.Int(0), args.Error(1)
}
func (m *MockBotClient) SetMenuCommands(ctx context.Context, chatID int64, isAdmin bool) error {
	args := m.Called(ctx, chatID, isAdmin)
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

// MockMessageHandler is a mock "plugin" for text
type MockMessageHandler struct {
	mock.Mock
}

func (m *MockMessageHandler) Handle(ctx context.Context, update *ports.BotUpdate, user *domain.User) error {
	args := m.Called(ctx, update, user)
	return args.Error(0)
}

// --- Tests ---

func TestRouter_HandleUpdate_Command(t *testing.T) {
	// 1. Setup
	ctx := context.Background()
	nopLogger := zerolog.Nop()
	mockUserRepo := new(MockUserRepository)
	mockBotClient := new(MockBotClient)

	router := NewCustomerRouter(mockUserRepo, mockBotClient, &nopLogger)

	// Create a mock handler for /start
	startHandler := new(MockCommandHandler)
	startHandler.On("Command").Return("start")
	startHandler.On("Handle").Return(nil).Once()

	// Create a mock handler for /help
	helpHandler := new(MockCommandHandler)
	helpHandler.On("Command").Return("help")

	// 2. Register handlers
	router.RegisterCommandHandler(startHandler)
	router.RegisterCommandHandler(helpHandler)

	// 3. Create a fake Telegram update
	fakeUpdate := &tgbotapi.Update{
		UpdateID: 123,
		Message: &tgbotapi.Message{
			MessageID: 456,
			From:      &tgbotapi.User{ID: 789, UserName: "testuser"},
			Chat:      &tgbotapi.Chat{ID: 1000},
			Text:      "/start",
			Entities: []tgbotapi.MessageEntity{
				{Type: "bot_command", Offset: 0, Length: 6},
			},
		},
	}

	// 4. We MUST add an expectation for the GetByTelegramID call
	// which happens *before* the command is routed.
	// For /start, the user might be nil, which is fine.
	mockUserRepo.On("GetByTelegramID", mock.Anything, int64(789)).Return(nil, nil).Once()

	// 5. Run the handler
	router.HandleUpdate(ctx, fakeUpdate)

	// 6. Assert expectations
	mockUserRepo.AssertExpectations(t)
	startHandler.AssertExpectations(t)
	helpHandler.AssertNotCalled(t, "Handle")
}

func TestRouter_HandleUpdate_Callback(t *testing.T) {
	// 1. Setup
	ctx := context.Background()
	nopLogger := zerolog.Nop()
	mockUserRepo := new(MockUserRepository)
	mockBotClient := new(MockBotClient)

	router := NewCustomerRouter(mockUserRepo, mockBotClient, &nopLogger)

	// Create a mock User (callbacks require an existing user)
	testUser := &domain.User{ID: uuid.New(), State: domain.StateAwaitingPolicyApproval}

	// Create mock handlers
	policyHandler := new(MockCallbackHandler)
	policyHandler.On("Prefix").Return("policy_")
	// We must now expect the user object
	policyHandler.On("Handle", mock.Anything, mock.AnythingOfType("*ports.BotUpdate"), testUser).Return(nil).Once()

	// 2. Register handlers
	router.RegisterCallbackHandler(policyHandler)

	// 3. Create a fake Telegram update
	callbackData := "policy_accept"
	fakeUpdate := &tgbotapi.Update{
		UpdateID: 124,
		CallbackQuery: &tgbotapi.CallbackQuery{
			ID:   "cb_id_1",
			From: &tgbotapi.User{ID: 789, UserName: "testuser"},
			Message: &tgbotapi.Message{
				MessageID: 456,
				Chat:      &tgbotapi.Chat{ID: 1000},
			},
			Data: callbackData,
		},
	}

	// 4. We MUST add an expectation for the GetByTelegramID call
	mockUserRepo.On("GetByTelegramID", mock.Anything, int64(789)).Return(testUser, nil).Once()

	// 5. Run the handler
	router.HandleUpdate(ctx, fakeUpdate)

	// 6. Assert expectations
	mockUserRepo.AssertExpectations(t)
	policyHandler.AssertExpectations(t)
}

// TestRouter_HandleUpdate_Text_NewUser
func TestRouter_HandleUpdate_Text_NewUser(t *testing.T) {
	// 1. Setup
	ctx := context.Background()
	nopLogger := zerolog.Nop()
	mockUserRepo := new(MockUserRepository)
	mockBotClient := new(MockBotClient)

	router := NewCustomerRouter(mockUserRepo, mockBotClient, &nopLogger)

	// 2. Create a fake Telegram update
	fakeUpdate := &tgbotapi.Update{
		UpdateID: 123,
		Message: &tgbotapi.Message{
			MessageID: 456,
			From:      &tgbotapi.User{ID: 789, UserName: "testuser"},
			Chat:      &tgbotapi.Chat{ID: 1000},
			Text:      "hello world", // Not a command
		},
	}

	// 3. Define Expectations
	// We expect the router to fetch the user and find nil
	mockUserRepo.On("GetByTelegramID", mock.Anything, int64(789)).Return(nil, nil).Once()
	// We expect the router to send a "please /start" message
	mockBotClient.On("SendMessage", mock.Anything, mock.AnythingOfType("ports.SendMessageParams")).Return(0, nil).Once()

	// 4. Run the handler
	router.HandleUpdate(ctx, fakeUpdate)

	// 5. Assert
	mockUserRepo.AssertExpectations(t)
	mockBotClient.AssertExpectations(t)
}

func TestRouter_HandleUpdate_StateRouting(t *testing.T) {
	// Tests that a text message from a known user is correctly routed to the MessageHandler

	// 1. Setup
	ctx := context.Background()
	nopLogger := zerolog.Nop()
	mockUserRepo := new(MockUserRepository)
	mockBotClient := new(MockBotClient)

	router := NewCustomerRouter(mockUserRepo, mockBotClient, &nopLogger)

	// Create and register the mock MessageHandler
	messageHandler := new(MockMessageHandler)
	router.SetMessageHandler(messageHandler)

	// 2. Create a fake User
	testUser := &domain.User{
		ID:    uuid.New(),
		State: domain.StateAwaitingFirstName, // User is in a state
	}

	// 3. Create a fake Telegram update
	fakeUpdate := &tgbotapi.Update{
		UpdateID: 123,
		Message: &tgbotapi.Message{
			MessageID: 456,
			From:      &tgbotapi.User{ID: 789, UserName: "testuser"},
			Chat:      &tgbotapi.Chat{ID: 1000},
			Text:      "Moein", // The user's first name
		},
	}

	// 4. Define Expectations
	// We expect the router to fetch the user
	mockUserRepo.On("GetByTelegramID", mock.Anything, int64(789)).Return(testUser, nil).Once()
	// We expect the router to call the MessageHandler
	messageHandler.On("Handle", mock.Anything, mock.AnythingOfType("*ports.BotUpdate"), testUser).Return(nil).Once()

	// 5. Run the handler
	router.HandleUpdate(ctx, fakeUpdate)

	// 6. Assert
	mockUserRepo.AssertExpectations(t)
	messageHandler.AssertExpectations(t)
	// We assert the botClient was *not* called by the router
	mockBotClient.AssertNotCalled(t, "SendMessage", mock.Anything, mock.Anything)
}

func TestRouter_HandleUpdate_UnhandledText(t *testing.T) {
	// 1. Setup
	ctx := context.Background()
	nopLogger := zerolog.Nop() // Logs are discarded
	mockUserRepo := new(MockUserRepository)
	mockBotClient := new(MockBotClient)
	router := NewCustomerRouter(mockUserRepo, mockBotClient, &nopLogger)

	// 2. Create a fake Telegram update
	fakeUpdate := &tgbotapi.Update{
		UpdateID: 123,
		Message: &tgbotapi.Message{
			MessageID: 456,
			From:      &tgbotapi.User{ID: 789, UserName: "testuser"},
			Chat:      &tgbotapi.Chat{ID: 1000},
			Text:      "hello world", // Not a command
		},
	}
	// 3. Define Expectations
	// We expect the router to fetch the user
	mockUserRepo.On("GetByTelegramID", mock.Anything, int64(789)).Return(nil, nil).Once()
	// We expect the router to send a "please /start" message
	mockBotClient.On("SendMessage", mock.Anything, mock.Anything).Return(0, nil).Once()
	// 4. Run the handler
	router.HandleUpdate(ctx, fakeUpdate)

	// 5. Assert
	mockUserRepo.AssertExpectations(t)
	mockBotClient.AssertExpectations(t)
}

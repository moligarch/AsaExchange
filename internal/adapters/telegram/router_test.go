package telegram

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
func (m *MockCallbackHandler) Handle(ctx context.Context, update *ports.BotUpdate) error {
	args := m.Called()
	return args.Error(0)
}

// --- Tests ---

func TestRouter_HandleUpdate_Command(t *testing.T) {
	// 1. Setup
	ctx := context.Background()
	nopLogger := zerolog.Nop()
	mockUserRepo := new(MockUserRepository)
	router := NewRouter(mockUserRepo, &nopLogger)

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
				{
					Type:   "bot_command",
					Offset: 0,
					Length: 6, // Length of "/start"
				},
			},
		},
	}

	// 4. Run the handler
	router.HandleUpdate(ctx, fakeUpdate)

	// 5. Assert expectations
	startHandler.AssertExpectations(t)
	helpHandler.AssertNotCalled(t, "Handle")
}

func TestRouter_HandleUpdate_Callback(t *testing.T) {
	// 1. Setup
	ctx := context.Background()
	nopLogger := zerolog.Nop()
	mockUserRepo := new(MockUserRepository)
	router := NewRouter(mockUserRepo, &nopLogger)

	// Create mock handlers
	bidHandler := new(MockCallbackHandler)
	bidHandler.On("Prefix").Return("bid_")
	bidHandler.On("Handle").Return(nil).Once()

	cancelHandler := new(MockCallbackHandler)
	cancelHandler.On("Prefix").Return("cancel_")

	// 2. Register handlers
	router.RegisterCallbackHandler(bidHandler)
	router.RegisterCallbackHandler(cancelHandler)

	// 3. Create a fake Telegram update
	callbackData := "bid_123-abc"
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

	// 4. Run the handler
	router.HandleUpdate(ctx, fakeUpdate)

	// 5. Assert expectations
	bidHandler.AssertExpectations(t)
	cancelHandler.AssertNotCalled(t, "Handle")
}

func TestRouter_HandleUpdate_UnhandledText(t *testing.T) {
	// 1. Setup
	ctx := context.Background()
	nopLogger := zerolog.Nop() // Logs are discarded
	mockUserRepo := new(MockUserRepository)
	router := NewRouter(mockUserRepo, &nopLogger)

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

	// 4. Run the handler
	router.HandleUpdate(ctx, fakeUpdate)
	// No assertions needed
}

package dto

type RegisterRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Nickname string `json:"nickname" binding:"required,min=1,max=64"`
	Password string `json:"password" binding:"required,min=8,max=128"`
}

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=8,max=128"`
	DeviceID string `json:"deviceId" binding:"omitempty,min=1,max=128"`
}

type RefreshRequest struct {
	RefreshToken string `json:"refreshToken" binding:"required"`
	DeviceID     string `json:"deviceId" binding:"omitempty,min=1,max=128"`
}

type RegisterData struct {
	UserID int64 `json:"userId"`
}

type LoginData struct {
	AccessToken  string      `json:"accessToken"`
	RefreshToken string      `json:"refreshToken"`
	ExpiresIn    int64       `json:"expiresIn"`
	User         LoginUserVO `json:"user"`
}

type RefreshData struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
	ExpiresIn    int64  `json:"expiresIn"`
}

type LoginUserVO struct {
	ID       int64  `json:"id"`
	Nickname string `json:"nickname"`
}

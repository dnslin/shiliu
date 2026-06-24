package v1

type InitializeRequest struct {
	Username string `json:"username" binding:"required" example:"shiliu"`
	Password string `json:"password" binding:"required" example:"123456789012"`
}

type InitializationStatusResponseData struct {
	Initialized bool `json:"initialized"`
}

type InitializationStatusResponse struct {
	Response
	Data InitializationStatusResponseData
}

type LoginRequest struct {
	Username string `json:"username" binding:"required" example:"shiliu"`
	Password string `json:"password" binding:"required" example:"123456789012"`
}
type LoginResponseData struct {
	AccessToken string `json:"accessToken"`
}
type LoginResponse struct {
	Response
	Data LoginResponseData
}
type ChangePasswordRequest struct {
	OldPassword string `json:"oldPassword" binding:"required" example:"123456789012"`
	NewPassword string `json:"newPassword" binding:"required" minLength:"12" maxLength:"72" example:"1234567890123"`
}

type GetProfileResponseData struct {
	Id       uint   `json:"id"`
	Username string `json:"username" example:"shiliu"`
}
type GetProfileResponse struct {
	Response
	Data GetProfileResponseData
}

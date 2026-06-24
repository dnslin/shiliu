package v1

type RegisterRequest struct {
	Username string `json:"username" binding:"required" example:"shiliu"`
	Password string `json:"password" binding:"required" example:"123456789012"`
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

type GetProfileResponseData struct {
	Id       uint   `json:"id"`
	Username string `json:"username" example:"shiliu"`
}
type GetProfileResponse struct {
	Response
	Data GetProfileResponseData
}

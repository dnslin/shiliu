package v1

var (
	// common errors
	ErrSuccess             = newError(0, "ok")
	ErrBadRequest          = newError(400, "Bad Request")
	ErrUnauthorized        = newError(401, "Unauthorized")
	ErrNotFound            = newError(404, "Not Found")
	ErrInternalServerError = newError(500, "Internal Server Error")

	// auth/account errors: 1000-1999
	ErrEmailAlreadyUse           = newError(1001, "The email is already in use.")
	ErrAccountNotInitialized     = newError(1002, "account not initialized")
	ErrAccountAlreadyInitialized = newError(1003, "account already initialized")
	ErrInvalidCredentials        = newError(1004, "invalid credentials")
	ErrAccountLocked             = newError(1005, "account locked")

	// feed/subscription errors: 2000-2999
	ErrFeedInvalidURL      = newError(2001, "invalid feed url")
	ErrFeedFetchFailed     = newError(2002, "feed fetch failed")
	ErrFeedParseFailed     = newError(2003, "feed parse failed")
	ErrFeedAlreadyExists   = newError(2004, "feed already exists")
	ErrFeedFetchInProgress = newError(2005, "feed fetch in progress")

	// content/inbox/search errors: 3000-3999
	ErrContentItemNotFound  = newError(3001, "content item not found")
	ErrInvalidContentFilter = newError(3002, "invalid content filter")

	// tag/folder errors: 4000-4999
	ErrTagAlreadyExists    = newError(4001, "tag already exists")
	ErrTagNotFound         = newError(4002, "tag not found")
	ErrFolderAlreadyExists = newError(4003, "folder already exists")
	ErrFolderNotFound      = newError(4004, "folder not found")

	// AI summary/service errors: 5000-5999
	ErrAIConfigMissing     = newError(5001, "ai config missing")
	ErrAISummaryInProgress = newError(5002, "ai summary in progress")
	ErrAIInsufficientText  = newError(5003, "ai insufficient text")
	ErrAISummaryFailed     = newError(5004, "ai summary failed")

	// export/import errors: 6000-6999
	ErrExportFailed     = newError(6001, "export failed")
	ErrOPMLInvalid      = newError(6002, "opml invalid")
	ErrOPMLImportFailed = newError(6003, "opml import failed")
)

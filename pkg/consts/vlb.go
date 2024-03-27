package consts

const (
	DEFAULT_LB_PREFIX_NAME = "vks" // "vks" is abbreviated of "cluster"

	DEFAULT_MEMBER_BACKUP_ROLE = false

	DEFAULT_PORTAL_NAME_LENGTH        = 50  // All the name must be less than 50 characters
	DEFAULT_PORTAL_DESCRIPTION_LENGTH = 255 // All the description must be less than 255 characters
	DEFAULT_MEMBER_WEIGHT             = 1
	DEFAULT_VLB_ID_PIECE_START_INDEX  = 8
	DEFAULT_VLB_ID_PIECE_LENGTH       = 6

	DEFAULT_HASH_NAME_LENGTH    = 5 // a unique hash name
	DEFAULT_NAME_DEFAULT_POOL   = "vks_default_pool"
	DEFAULT_PACKAGE_ID          = "lbp-f562b658-0fd4-4fa6-9c57-c1a803ccbf86"
	DEFAULT_HTTPS_LISTENER_NAME = "vks_https_listener"
	DEFAULT_HTTP_LISTENER_NAME  = "vks_http_listener"

	ACTIVE_LOADBALANCER_STATUS = "ACTIVE"
)

package account

type Account struct {
	ID       int64  `json:"id"`
	AuthId   string `json:"auth_id" db:"auth_id"`
	Username string `json:"username" db:"username"`
}

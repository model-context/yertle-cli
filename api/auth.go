package api

type SignInRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type SignInUser struct {
	Email         string `json:"email"`
	Sub           string `json:"sub"`
	EmailVerified bool   `json:"email_verified"`
	Name          string `json:"name"`
}

type SignInResponse struct {
	Message      string     `json:"message"`
	User         SignInUser `json:"user"`
	AccessToken  string     `json:"accessToken"`
	RefreshToken string     `json:"refreshToken"`
	ExpiresIn    int        `json:"expiresIn"`
}

type RefreshRequest struct {
	RefreshToken string `json:"refreshToken"`
}

type RefreshResponse struct {
	AccessToken string `json:"accessToken"`
	TokenType   string `json:"tokenType"`
	ExpiresIn   int    `json:"expiresIn"`
}

type VerifyRequest struct {
	Token string `json:"token"`
}

type VerifyResponse struct {
	Valid bool       `json:"valid"`
	User  SignInUser `json:"user,omitempty"`
}

func (c *Client) SignIn(email, password string) (*SignInResponse, error) {
	req := SignInRequest{Email: email, Password: password}
	var resp SignInResponse
	if err := c.Post("/auth/signin", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) RefreshToken(refreshToken string) (*RefreshResponse, error) {
	req := RefreshRequest{RefreshToken: refreshToken}
	var resp RefreshResponse
	if err := c.Post("/auth/refresh", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) VerifyToken(token string) (*VerifyResponse, error) {
	req := VerifyRequest{Token: token}
	var resp VerifyResponse
	if err := c.Post("/auth/verify", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

package flarerr

type flareRequest struct {
	Cmd        string `json:"cmd"`
	Url        string `json:"url"`
	MaxTimeout int    `json:"maxTimeout"`
}

type flareResponse struct {
	Status         string        `json:"status"`
	Message        string        `json:"message"`
	Solution       flareSolution `json:"solution"`
	StartTimestamp int64         `json:"startTimestamp"`
	EndTimestamp   int64         `json:"endTimestamp"`
	Version        string        `json:"version"`
}

type flareCookie struct {
	Domain   string `json:"domain"`
	Expiry   int64  `json:"expiry"`
	HttpOnly bool   `json:"httpOnly"`
	Name     string `json:"name"`
	Path     string `json:"path"`
	SameSite string `json:"sameSite"`
	Secure   bool   `json:"secure"`
	Value    string `json:"value"`
	Size     int    `json:"size"`
	Session  bool   `json:"session"`
	Expires  int64  `json:"expires"`
}

type flareSolution struct {
	Url       string        `json:"url"`
	Status    int           `json:"status"`
	Cookies   []flareCookie `json:"cookies"`
	UserAgent string        `json:"userAgent"`
	Response  string        `json:"response"`
}

package urlsinger

import (
	"fmt"
	goalone "github.com/bwmarrin/go-alone"
	"strings"
)

type Singer struct {
	Secret []byte
}

func (s *Singer) GenerateTokenFromString(data string) string {
	var urlToSign string

	crypt := goalone.New(s.Secret, goalone.Timestamp)
	if strings.Contains(data, "?") {
		urlToSign = fmt.Sprintf("%s&hash=", data)
	} else {
		urlToSign = fmt.Sprintf("%s?hash=", data)
	}

	tokenBytes := crypt.Sign([]byte(urlToSign))
	token := string(tokenBytes)
	return token
}

package util

import (
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
)

type Config struct {
	DBHost             string
	DBDriver           string
	DBUser             string
	DBPassword         string
	DBName             string
	DBPort             string
	BaseUrl            string
	TokenSymmetricKey  string
	FeUrl              string
	GoogleClientId     string
	GoogleClientSecret string
	GithubClientId     string
	GithubClientSecret string
	DomainName string
}

func LoadConfig(path string) (config Config, err error) {
	err = godotenv.Load(filepath.Join(path, ".env"))

	if err != nil {
		return
	}

	config.DBDriver = os.Getenv("DB_DRIVER")
	config.DBHost = os.Getenv("DB_HOST")
	config.DBUser = os.Getenv("DB_USER")
	config.DBPassword = os.Getenv("DB_PASSWORD")
	config.DBName = os.Getenv("DB_NAME")
	config.DBPort = os.Getenv("DB_PORT")
	config.BaseUrl = os.Getenv("BASE_URL")
	config.TokenSymmetricKey = os.Getenv("TOKEN_SYMMETRIC_KEY")
	config.FeUrl = os.Getenv("FE_URL")
	config.GoogleClientId = os.Getenv("GOOGLE_CLIENT_ID")
	config.GoogleClientSecret = os.Getenv("GOOGLE_CLIENT_SECRET")
	config.GithubClientId = os.Getenv("GITHUB_CLIENT_ID")
	config.GithubClientSecret = os.Getenv("GITHUB_CLIENT_SECRET")
	config.DomainName = os.Getenv("DOMAIN_NAME")

	return
}

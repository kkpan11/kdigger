package token

import (
	"os"

	"github.com/mtardy/kdigger/pkg/bucket"
)

const (
	bucketName        = "token"
	bucketDescription = "Token checks for the presence of a service account token in the filesystem."

	tokenPath = "/run/secrets/kubernetes.io/serviceaccount"
)

var bucketAliases = []string{"tokens", "tk"}
var config bucket.Config

type TokenBucket struct{}

func (n TokenBucket) Run() (bucket.Results, error) {
	res := bucket.NewResults(bucketName)
	if tokenFolderExist() {
		res.SetComment("A service account token is mounted.")

		err := res.SetHeaders([]string{"Namespace", "Token", "CA"})
		if err != nil {
			return bucket.Results{}, err
		}

		ns, err := readMountedData("namespace")
		if err != nil {
			return bucket.Results{}, err
		}
		t, err := readMountedData("token")
		if err != nil {
			return bucket.Results{}, err
		}
		ca, err := readMountedData("ca.crt")
		if err != nil {
			return bucket.Results{}, err
		}

		err = res.AddContent([]string{ns, t, ca})
		if err != nil {
			return bucket.Results{}, err
		}
	} else {
		res.SetComment("No service account token was found")
	}
	return *res, nil
}

func Register(b *bucket.Buckets) {
	b.Register(bucketName, bucketAliases, bucketDescription, func(config bucket.Config) (bucket.Interface, error) {
		return NewTokenBucket(config)
	})
}

func NewTokenBucket(c bucket.Config) (*TokenBucket, error) {
	if c.Client == nil {
		return nil, bucket.ErrMissingClient
	}
	config = c
	return &TokenBucket{}, nil
}

func tokenFolderExist() bool {
	_, err := os.Stat(tokenPath)
	return !os.IsNotExist(err)
}

func readMountedData(data string) (string, error) {
	b, err := os.ReadFile(tokenPath + "/" + data)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

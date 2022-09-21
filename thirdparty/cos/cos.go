package cos

import (
	"context"
	"github.com/toolkits/pkg/logger"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/tencentyun/cos-go-sdk-v5"
)

func DownloadCOSBucket(cosURL, secretID, secretKey, fileName string) (io.ReadCloser, error) {
	u, _ := url.Parse(cosURL)
	b := &cos.BaseURL{BucketURL: u}
	c := cos.NewClient(b, &http.Client{
		Timeout: 100 * time.Second,
		Transport: &cos.AuthorizationTransport{
			//SecretID: os.Getenv(secretID),
			SecretID: secretID,
			//SecretKey: os.Getenv(secretKey),
			SecretKey: secretKey,
		},
	})

	resp, err := c.Object.Get(context.Background(), fileName, nil)
	if err != nil {
		return nil, err
	}
	return resp.Body, nil
}

func Upload(cosURL, secretID, secretKey, bucketName string, file io.Reader) error {
	u, _ := url.Parse(cosURL)
	b := &cos.BaseURL{BucketURL: u}
	c := cos.NewClient(b, &http.Client{
		Timeout: 100 * time.Second,
		Transport: &cos.AuthorizationTransport{
			SecretID:  secretID,
			SecretKey: secretKey,
		},
	})
	_, err := c.Object.Put(context.Background(), bucketName, file, &cos.ObjectPutOptions{
		ACLHeaderOptions: &cos.ACLHeaderOptions{
			XCosACL: "public-read",
		},
	})
	if err != nil {
		return err
	}
	logger.Infof("upload file to bucket %s done", bucketName)
	return nil
}

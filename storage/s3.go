package storage

import (
	"bytes"
	"fmt"

	log "github.com/sirupsen/logrus"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/rapid7/pdf-renderer/cfg"
)

type s3Client struct {
	client     *s3.S3
	uploader   *s3manager.Uploader
	downloader *s3manager.Downloader
	bucket     string
}

// Variables needed to enforce singleton pattern for S3Client
var (
	client *s3Client
	s3err  error
)

// singleton client
func getS3Client() (*s3Client, error) {
	if client == nil {
		client, s3err = newS3Client()
		if s3err != nil {
			return nil, s3err
		}
	}
	return client, nil
}

func newS3Client() (*s3Client, error) {
	sess := session.Must(session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
	}))
	svc := s3.New(sess)

	// Attempt to create bucket
	bucketInput := &s3.CreateBucketInput{
		Bucket: aws.String(cfg.Config().S3Bucket()),
	}

	_, err := svc.CreateBucket(bucketInput)
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case s3.ErrCodeBucketAlreadyExists:
				log.Errorf(fmt.Sprintf("%s bucket already exits", cfg.Config().S3Bucket()))
			case s3.ErrCodeBucketAlreadyOwnedByYou:
				log.Errorf(fmt.Sprintf("%s bucket already created", cfg.Config().S3Bucket()))
			default:
				log.Error(aerr.Error())
			}
		} else {
			log.Error(err.Error())
		}
		return nil, err
	}

	u := s3manager.NewUploader(sess)
	d := s3manager.NewDownloader(sess)

	return &s3Client{
		client:     svc,
		uploader:   u,
		downloader: d,
		bucket:     cfg.Config().S3Bucket(),
	}, nil
}

type s3Object struct {
	fileName string
	s3client *s3Client
}

func NewS3Object(fileName string) (*s3Object, error) {
	client, err := getS3Client()
	if err != nil {
		return nil, err
	}

	return &s3Object{
		fileName: fileName,
		s3client: client,
	}, nil

}

func (o *s3Object) FileName() string {
	return o.fileName
}

func (o *s3Object) Write(data []byte) error {

	upParams := &s3manager.UploadInput{
		Bucket: aws.String(o.s3client.bucket),
		Key:    aws.String(o.FileName()),
		Body:   bytes.NewReader(data),
	}

	// Perform the upload with supplied params.
	_, err := o.s3client.uploader.Upload(upParams)
	if err != nil {
		return err
	}

	return nil
}

func (o *s3Object) Read() ([]byte, error) {
	buf := aws.NewWriteAtBuffer([]byte{})

	downParams := &s3.GetObjectInput{
		Bucket: aws.String(o.s3client.bucket),
		Key:    aws.String(o.FileName()),
	}

	_, err := o.s3client.downloader.Download(buf, downParams)

	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil

}

func (o *s3Object) Exists() bool {
	input := &s3.GetObjectInput{
		Bucket: aws.String(o.s3client.bucket),
		Key:    aws.String(o.FileName()),
	}

	_, err := o.s3client.client.GetObject(input)
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case s3.ErrCodeNoSuchKey:
				log.Errorf(fmt.Sprintf("%v error: the key '%v' does not exist", s3.ErrCodeNoSuchKey, o.FileName()))
			default:
				log.Error(aerr.Error())
			}
		} else {
			log.Error(err.Error())
		}
		return false
	}
	return true
}

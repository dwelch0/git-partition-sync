package pkg

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type s3ObjectInfo struct {
	Key       *string
	CommitSHA string
}

// processes response of ListObjectsV2 against aws api
// return is map of destination PID to s3ObjectInfo
// Context: within s3, our uploaded object keys are based64 encoded jsons
func (u *Uploader) getS3Keys(ctx context.Context) (map[string]*s3ObjectInfo, error) {
	ctxTimeout, cancel := context.WithTimeout(ctx, time.Second*5)
	defer cancel()

	res, err := u.s3Client.ListObjectsV2(ctxTimeout, &s3.ListObjectsV2Input{
		Bucket: &u.bucket,
	})
	if err != nil {
		return nil, err
	}

	s3ObjectInfos := make(map[string]*s3ObjectInfo)
	for _, obj := range res.Contents {
		// remove file extension before attempting decode
		// extension is .tar.age, split at first occurrence of .
		encodedKey := strings.SplitN(*obj.Key, ".", 2)[0]
		decodedBytes, err := base64.StdEncoding.DecodeString(encodedKey)
		if err != nil {
			return nil, err
		}
		var jsonKey DecodedKey
		err = json.Unmarshal(decodedBytes, &jsonKey)
		if err != nil {
			return nil, err
		}
		pid := fmt.Sprintf("%s/%s", jsonKey.Group, jsonKey.ProjectName)
		s3ObjectInfos[pid] = &s3ObjectInfo{
			Key:       obj.Key,
			CommitSHA: jsonKey.CommitSHA,
		}
	}
	return s3ObjectInfos, nil
}

// concurrently deletes objects from s3 sync bucket that are no longer needed
func (u *Uploader) removeOutdated(ctx context.Context, toDeleteKeys []*string) error {
	ctxTimeout, cancel := context.WithTimeout(ctx, time.Second*5)
	defer cancel()

	wg := &sync.WaitGroup{}
	ch := make(chan error)

	for _, key := range toDeleteKeys {
		wg.Add(1)
		go func(k *string) {
			defer wg.Done()

			_, err := u.s3Client.DeleteObject(ctxTimeout, &s3.DeleteObjectInput{
				Bucket: &u.bucket,
				Key:    k,
			})
			if err != nil {
				ch <- err
			}
		}(key)
	}

	go func() {
		wg.Wait()
		close(ch)
	}()

	for err := range ch {
		if err != nil {
			return err
		}
	}

	return nil
}

// cocurrently uploads latest encrypted tars to target s3 bucket
func (u *Uploader) uploadLatest(ctx context.Context, toUpdate []*GitSync, glCommits pidToCommit) error {
	ctxTimeout, cancel := context.WithTimeout(ctx, time.Second*10)
	defer cancel()

	wg := &sync.WaitGroup{}
	ch := make(chan error)
	// including sem due to following goroutines utilizing relatively expensive file io
	sem := make(chan struct{}, 20) // arbitary value. TODO: evaluate resource consumption and adjust

	for _, gs := range toUpdate {
		wg.Add(1)
		sem <- struct{}{} // block if 20 goroutines already running

		go func(gsync *GitSync) {
			defer func() { <-sem }() // release one from buffer
			defer wg.Done()          // must exec before sem release

			sourcePid := fmt.Sprintf("%s/%s", gsync.Source.Group, gsync.Source.ProjectName)

			jsonStruct := &DecodedKey{
				Group:       gsync.Destination.Group,
				ProjectName: gsync.Destination.ProjectName,
				CommitSHA:   glCommits[sourcePid],
				Branch:      gsync.Destination.Branch,
			}

			jsonBytes, err := json.Marshal(jsonStruct)
			if err != nil {
				ch <- err
				return
			}

			encodedJsonStr := base64.StdEncoding.EncodeToString(jsonBytes)
			objKey := fmt.Sprintf("%s.tar.age", encodedJsonStr)

			f, err := os.Open(gsync.encryptPath)
			defer f.Close()

			_, err = u.s3Client.PutObject(ctxTimeout, &s3.PutObjectInput{
				Bucket: &u.bucket,
				Key:    &objKey,
				Body:   f,
			})

			if err != nil {
				ch <- err
				return
			}
		}(gs)
	}

	go func() {
		wg.Wait()
		close(ch)
		close(sem)
	}()

	for err := range ch {
		if err != nil {
			return err
		}
	}

	return nil
}

package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/minio/minio-go/v7/pkg/tags"

	miniogo "github.com/minio/minio-go/v7"
)

// contentMessage container for content message structure.
type contentMessage struct {
	Status   string    `json:"status"`
	Filetype string    `json:"type"`
	Time     time.Time `json:"lastModified"`
	Size     int64     `json:"size"`
	Key      string    `json:"key"`
	ETag     string    `json:"etag"`
	URL      string    `json:"url,omitempty"`

	VersionID      string `json:"versionId,omitempty"`
	VersionOrd     int    `json:"versionOrdinal,omitempty"`
	VersionIndex   int    `json:"versionIndex,omitempty"`
	IsDeleteMarker bool   `json:"isDeleteMarker,omitempty"`
}

type copyState struct {
	objectCh  chan contentMessage
	failedCh  chan contentMessage
	successCh chan contentMessage
	count     uint64
	failCnt   uint64
	wg        sync.WaitGroup
}

func (m *copyState) queueUploadTask(obj contentMessage) {
	m.objectCh <- obj
}

var (
	cpState        *copyState
	copyConcurrent = 100
)

func newCopyState(ctx context.Context) *copyState {
	if runtime.GOMAXPROCS(0) > copyConcurrent {
		copyConcurrent = runtime.GOMAXPROCS(0)
	}
	cp := &copyState{
		objectCh:  make(chan contentMessage, copyConcurrent),
		failedCh:  make(chan contentMessage, copyConcurrent),
		successCh: make(chan contentMessage, copyConcurrent),
	}

	return cp
}

// Increase count processed
func (m *copyState) incCount() {
	atomic.AddUint64(&m.count, 1)
}

// Get total count processed
func (m *copyState) getCount() uint64 {
	return atomic.LoadUint64(&m.count)
}

// Increase count failed
func (m *copyState) incFailCount() {
	atomic.AddUint64(&m.failCnt, 1)
}

// Get total count failed
func (m *copyState) getFailCount() uint64 {
	return atomic.LoadUint64(&m.failCnt)
}

// addWorker creates a new worker to process tasks
func (m *copyState) addWorker(ctx context.Context) {
	m.wg.Add(1)
	// Add a new worker.
	go func() {
		defer m.wg.Done()
		for {
			select {
			case <-ctx.Done():
				return
			case obj, ok := <-m.objectCh:
				if !ok {
					return
				}
				logDMsg(fmt.Sprintf("Copying...%v", obj), nil)
				if err := copyObject(ctx, obj); err != nil {
					m.incFailCount()
					logMsg(fmt.Sprintf("error moving object %v: %w", obj, err))
					m.failedCh <- obj
					continue
				}
				logMsg(fmt.Sprintf("Successully copied %v", obj))
				m.successCh <- obj
				m.incCount()
			}
		}
	}()
}

func (m *copyState) finish(ctx context.Context) {
	time.Sleep(100 * time.Millisecond)
	close(m.objectCh)
	m.wg.Wait() // wait on workers to finish
	close(m.failedCh)
	close(m.successCh)

	if !dryRun {
		logMsg(fmt.Sprintf("Copied %d objects, %d failures", m.getCount(), m.getFailCount()))
	}
}
func (m *copyState) init(ctx context.Context) {
	if m == nil {
		return
	}
	for i := 0; i < copyConcurrent; i++ {
		m.addWorker(ctx)
	}
	go func() {
		f, err := os.OpenFile(path.Join(dirPath, failCopyFile+time.Now().Format(".01-02-2006-15-04-05")), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
		if err != nil {
			logDMsg("could not create "+failCopyFile, err)
			return
		}
		fwriter := bufio.NewWriter(f)
		defer fwriter.Flush()
		defer f.Close()

		s, err := os.OpenFile(path.Join(dirPath, successCopyFile+time.Now().Format(".01-02-2006-15-04-05")), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
		if err != nil {
			logDMsg("could not create "+successCopyFile, err)
			return
		}
		swriter := bufio.NewWriter(s)
		defer swriter.Flush()
		defer s.Close()

		for {
			select {
			case <-ctx.Done():
				return
			case obj, ok := <-m.failedCh:
				if !ok {
					return
				}
				bytes, _ := json.Marshal(obj)
				if _, err := f.WriteString(string(bytes) + "\n"); err != nil {
					logMsg(fmt.Sprintf("Error writing to copy_fails.txt for %v : %w", obj, err))
					os.Exit(1)
				}
			case obj, ok := <-m.successCh:
				if !ok {
					return
				}
				logMsg(fmt.Sprintf("Writing %v", obj))

				bytes, _ := json.Marshal(obj)
				if _, err := s.WriteString(string(bytes) + "\n"); err != nil {
					logMsg(fmt.Sprintf("Error writing to copy_success.txt for %v : %w", obj, err))
					os.Exit(1)
				}

			}
		}
	}()
}

func copyObject(ctx context.Context, m contentMessage) error {
	object := m.Key
	minioClient.TraceOn(os.Stdout)
	_, err := minioClient.StatObject(ctx, minioDstBucket, object, miniogo.StatObjectOptions{
		VersionID: m.VersionID,
		Internal: miniogo.AdvancedGetOptions{
			ReplicationProxyRequest: "false",
		}})
	if err == nil {
		return err // already replicated
	}
	if m.IsDeleteMarker {
		if strings.Contains(err.Error(), "method is not allowed") {
			logMsg(fmt.Sprintf("Dst already has this delete marker version -  %s (%s)", object, m.VersionID))
			return nil // count as success
		}
		rmErr := minioClient.RemoveObject(ctx, minioDstBucket, object, miniogo.RemoveObjectOptions{
			VersionID: m.VersionID,
			Internal: miniogo.AdvancedRemoveOptions{
				ReplicationDeleteMarker: m.IsDeleteMarker,
				ReplicationMTime:        m.Time,
				ReplicationStatus:       miniogo.ReplicationStatusReplica,
				ReplicationRequest:      true, // always set this to distinguish between `mc mirror` replication and serverside
			},
		})
		return rmErr
	}

	gopts := miniogo.GetObjectOptions{
		VersionID: m.VersionID,
		Internal: miniogo.AdvancedGetOptions{
			ReplicationProxyRequest: "true",
		},
	}
	// Make sure to match ETag when proxying.
	if err := gopts.SetMatchETag(m.ETag); err != nil {
		return nil
	}
	c := miniogo.Core{Client: minioSrcClient}
	obj, objInfo, _, err := c.GetObject(ctx, minioSrcBucket, object, gopts)
	if err != nil {
		return nil
	}
	defer obj.Close()

	_, err = minioClient.PutObject(ctx, minioDstBucket, object, obj, objInfo.Size, putReplicationOpts(ctx, objInfo))
	if err != nil {
		logDMsg("upload to minio client failed for "+object, err)
		return err
	}
	logDMsg("Uploaded "+object+" successfully", nil)
	return nil
}

const (
	ContentEncoding = "Content-Encoding"
)

func putReplicationOpts(ctx context.Context, objInfo miniogo.ObjectInfo) (putOpts miniogo.PutObjectOptions) {
	meta := make(map[string]string)
	for k, v := range objInfo.UserMetadata {
		meta[k] = v
	}
	enc, ok := objInfo.Metadata[ContentEncoding]
	if !ok {
		enc = objInfo.Metadata[strings.ToLower(ContentEncoding)]
	}
	sc := objInfo.StorageClass

	putOpts = miniogo.PutObjectOptions{
		UserMetadata:    meta,
		ContentType:     objInfo.ContentType,
		ContentEncoding: strings.Join(enc, ","),
		StorageClass:    sc,
		Internal: miniogo.AdvancedPutOptions{
			SourceVersionID:    objInfo.VersionID,
			ReplicationStatus:  miniogo.ReplicationStatusReplica,
			SourceMTime:        objInfo.LastModified,
			SourceETag:         objInfo.ETag,
			ReplicationRequest: true, // always set this to distinguish between `mc mirror` replication and serverside
		},
	}
	if objInfo.UserTags != nil {
		tags, _ := tags.MapToObjectTags(objInfo.UserTags)
		if tags != nil {
			putOpts.UserTags = tags.ToMap()
		}
	}
	return
}

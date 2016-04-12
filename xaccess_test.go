package xaccess

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"runtime"
	"testing"
	"time"

	"github.com/rs/xhandler"
	"github.com/rs/xlog"
	"github.com/stretchr/testify/assert"
	"golang.org/x/net/context"
)

type fakeContext struct {
	err error
}

func (c fakeContext) Err() error {
	return c.err
}

func (c fakeContext) Deadline() (deadline time.Time, ok bool) {
	return time.Now(), true
}

func (c fakeContext) Done() <-chan struct{} {
	return make(chan struct{})
}

func (c fakeContext) Value(key interface{}) interface{} {
	return nil
}

type recorderOutput struct {
	last map[string]interface{}
}

func (o *recorderOutput) Write(fields map[string]interface{}) (err error) {
	o.last = map[string]interface{}{}
	for k, v := range fields {
		o.last[k] = v
	}
	return nil
}

func TestResponseStatus(t *testing.T) {
	assert.Equal(t, "ok", responseStatus(fakeContext{err: nil}, http.StatusOK))
	assert.Equal(t, "canceled", responseStatus(fakeContext{err: context.Canceled}, http.StatusOK))
	assert.Equal(t, "timeout", responseStatus(fakeContext{err: context.DeadlineExceeded}, http.StatusOK))
	assert.Equal(t, "error", responseStatus(fakeContext{err: nil}, http.StatusFound))
}

func TestNewHandler(t *testing.T) {
	h := NewHandler()(xhandler.HandlerFuncC(func(ctx context.Context, w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
		w.Write([]byte{'1', '2', '3'})
	}))
	o := &recorderOutput{}
	l := xlog.NewHandler(xlog.Config{
		Output: o,
	})
	r, _ := http.NewRequest("GET", "/path", nil)
	w := httptest.NewRecorder()
	l(h).ServeHTTPC(context.Background(), w, r)
	runtime.Gosched()
	for i := 0; len(o.last) == 0 && i < 100; i++ {
		time.Sleep(10 * time.Millisecond)
	}
	if assert.NotNil(t, o.last) {
		assert.Equal(t, 3, o.last["size"])
		assert.Equal(t, "info", o.last["level"])
		assert.Equal(t, "access", o.last["type"])
		assert.Equal(t, "ok", o.last["status"])
		assert.Equal(t, 202, o.last["status_code"])
		assert.Equal(t, "GET /path 202", o.last["message"])
		assert.NotEmpty(t, o.last["duration"])
		assert.NotEmpty(t, o.last["time"])
	}

	xx := string(bytes.Repeat([]byte{'x'}, 150))
	r, _ = http.NewRequest("GET", "/"+xx, nil)
	w = httptest.NewRecorder()
	l(h).ServeHTTPC(context.Background(), w, r)
	runtime.Gosched()
	for i := 0; len(o.last) == 0 && i < 100; i++ {
		time.Sleep(10 * time.Millisecond)
	}
	if assert.NotNil(t, o.last) {
		assert.Equal(t, "GET /"+xx[:48]+"..."+xx[:49]+" 202", o.last["message"])
	}
}

func TestEllipsize(t *testing.T) {
	assert.Equal(t, "", ellipsize("", 10))
	assert.Equal(t, "s...g", ellipsize("somestring", 5))
	assert.Equal(t, "somestring", ellipsize("somestring", 10))
	assert.Equal(t, "som...ing", ellipsize("somestring", 9))
	assert.Equal(t, "", ellipsize("somestring", 0))
	assert.Equal(t, ".", ellipsize("somestring", 1))
	assert.Equal(t, "..", ellipsize("somestring", 2))
	assert.Equal(t, "...", ellipsize("somestring", 3))
	assert.Equal(t, "s...g", ellipsize("somestring", 4))
	assert.Equal(t, "s...g", ellipsize("somestring", 5))
}

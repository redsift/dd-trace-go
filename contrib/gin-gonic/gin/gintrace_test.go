package gin

import (
	"errors"
	"fmt"
	"html/template"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/DataDog/dd-trace-go/tracer"
	"github.com/DataDog/dd-trace-go/tracer/ext"
	"github.com/DataDog/dd-trace-go/tracer/tracertest"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func init() {
	gin.SetMode(gin.ReleaseMode) // silence annoying log msgs
}

func TestChildSpan(t *testing.T) {
	assert := assert.New(t)
	testTracer, _ := tracertest.GetTestTracer()
	tracer.DefaultTracer = testTracer

	router := gin.New()
	router.Use(Middleware("foobar"))
	router.GET("/user/:id", func(c *gin.Context) {
		_, ok := tracer.SpanFromContext(c.Request.Context())
		assert.True(ok)
	})

	r := httptest.NewRequest("GET", "/user/123", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, r)
}

func TestTrace200(t *testing.T) {
	assert := assert.New(t)
	testTracer, testTransport := tracertest.GetTestTracer()
	tracer.DefaultTracer = testTracer

	router := gin.New()
	router.Use(Middleware("foobar"))
	router.GET("/user/:id", func(c *gin.Context) {
		// assert we patch the span on the request context.
		span, ok := tracer.SpanFromContext(c.Request.Context())
		assert.True(ok)
		span.SetMeta("test.gin", "ginny")
		assert.Equal(span.Service, "foobar")
		id := c.Param("id")
		c.Writer.Write([]byte(id))
	})

	r := httptest.NewRequest("GET", "/user/123", nil)
	w := httptest.NewRecorder()

	// do and verify the request
	router.ServeHTTP(w, r)
	response := w.Result()
	assert.Equal(response.StatusCode, 200)

	// verify traces look good
	testTracer.ForceFlush()
	traces := testTransport.Traces()
	assert.Len(traces, 1)
	spans := traces[0]
	assert.Len(spans, 1)
	if len(spans) < 1 {
		t.Fatalf("no spans")
	}
	s := spans[0]
	assert.Equal(s.Service, "foobar")
	assert.Equal(s.Name, "http.request")
	// FIXME[matt] would be much nicer to have "/user/:id" here
	assert.True(strings.Contains(s.Resource, "gin.TestTrace200"))
	assert.Equal(s.GetMeta("test.gin"), "ginny")
	assert.Equal(s.GetMeta("http.status_code"), "200")
	assert.Equal(s.GetMeta("http.method"), "GET")
	assert.Equal(s.GetMeta("http.url"), "/user/123")
}

func TestDisabled(t *testing.T) {
	assert := assert.New(t)
	testTracer, testTransport := tracertest.GetTestTracer()
	testTracer.SetEnabled(false)
	tracer.DefaultTracer = testTracer

	router := gin.New()
	router.Use(Middleware("foobar"))
	router.GET("/ping", func(c *gin.Context) {
		_, ok := tracer.SpanFromContext(c.Request.Context())
		assert.False(ok)
		c.Writer.Write([]byte("ok"))
	})

	r := httptest.NewRequest("GET", "/ping", nil)
	w := httptest.NewRecorder()

	// do and verify the request
	router.ServeHTTP(w, r)
	response := w.Result()
	assert.Equal(response.StatusCode, 200)

	// verify traces look good
	testTracer.ForceFlush()
	spans := testTransport.Traces()
	assert.Len(spans, 0)
}

func TestError(t *testing.T) {
	assert := assert.New(t)
	testTracer, testTransport := tracertest.GetTestTracer()
	tracer.DefaultTracer = testTracer

	// setup
	router := gin.New()
	router.Use(Middleware("foobar"))

	// a handler with an error and make the requests
	router.GET("/err", func(c *gin.Context) {
		c.AbortWithError(500, errors.New("oh no"))
	})
	r := httptest.NewRequest("GET", "/err", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)
	response := w.Result()
	assert.Equal(response.StatusCode, 500)

	// verify the errors and status are correct
	testTracer.ForceFlush()
	traces := testTransport.Traces()
	assert.Len(traces, 1)
	spans := traces[0]
	assert.Len(spans, 1)
	if len(spans) < 1 {
		t.Fatalf("no spans")
	}
	s := spans[0]
	assert.Equal(s.Service, "foobar")
	assert.Equal(s.Name, "http.request")
	assert.Equal(s.GetMeta(ext.HTTPCode), "500")
	assert.Equal(s.GetMeta(ext.ErrorMsg), "oh no")
	assert.Equal(s.Error, int32(1))
}

func TestHTML(t *testing.T) {
	assert := assert.New(t)
	testTracer, testTransport := tracertest.GetTestTracer()
	tracer.DefaultTracer = testTracer

	// setup
	router := gin.New()
	router.Use(Middleware("foobar"))

	// add a template
	tmpl := template.Must(template.New("hello").Parse("hello {{.}}"))
	router.SetHTMLTemplate(tmpl)

	// a handler with an error and make the requests
	router.GET("/hello", func(c *gin.Context) {
		HTML(c, 200, "hello", "world")
	})
	r := httptest.NewRequest("GET", "/hello", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)
	response := w.Result()
	assert.Equal(response.StatusCode, 200)
	assert.Equal("hello world", w.Body.String())

	// verify the errors and status are correct
	testTracer.ForceFlush()
	traces := testTransport.Traces()
	assert.Len(traces, 1)
	spans := traces[0]
	assert.Len(spans, 2)
	for _, s := range spans {
		assert.Equal(s.Service, "foobar")
	}

	var tspan *tracer.Span
	for _, s := range spans {
		// we need to pick up the span we're searching for, as the
		// order is not garanteed within the buffer
		if s.Name == "gin.render.html" {
			tspan = s
		}
	}
	assert.NotNil(tspan, "we should have found a span with name gin.render.html")
	assert.Equal(tspan.GetMeta("go.template"), "hello")
	fmt.Println(spans)
}

func TestGetSpanNotInstrumented(t *testing.T) {
	assert := assert.New(t)
	router := gin.New()
	router.GET("/ping", func(c *gin.Context) {
		// Assert we don't have a span on the context.
		_, ok := tracer.SpanFromContext(c.Request.Context())
		assert.False(ok)
		c.Writer.Write([]byte("ok"))
	})
	r := httptest.NewRequest("GET", "/ping", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)
	response := w.Result()
	assert.Equal(response.StatusCode, 200)
}

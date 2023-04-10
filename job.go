package scrapemate

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"time"

	"github.com/playwright-community/playwright-go"
)

var _ IJob = (*Job)(nil)

// IJob is a job to be processed by the scrapemate
type IJob interface {
	fmt.Stringer
	// GetID returns the unique identifier of the job.
	GetID() string
	// GetMethod returns the http method to use
	GetMethod() string
	// GetBody returns the body of the request
	GetBody() []byte
	// GetURL returns the url to request
	GetURL() string
	// GetHeaders returns the headers to use
	GetHeaders() map[string]string
	// GetUrlParams returns the url params to use
	GetUrlParams() map[string]string
	// GetTimeout returns the timeout of the job
	GetTimeout() time.Duration
	// GetPriority returns the priority of the job
	GetPriority() int
	// CheckResponse checks the response of the job
	DoCheckResponse(resp Response) bool
	// GetActionOnResponse returns the action to perform on the response
	GetRetryPolicy() RetryPolicy
	// GetMaxRetries returns the max retries of the job
	GetMaxRetries() int
	// Process processes the job
	Process(ctx context.Context, resp Response) (any, []IJob, error)
	// GetMaxRetryDelay returns the delay to wait before retrying
	GetMaxRetryDelay() time.Duration
	BrowserActions(browser playwright.Browser) Response
	// DoScreenshot takes a screenshot of the page
	// Only works if the scraper uses jsfetcher
	DoScreenshot() bool
	// GetCacheKey returns the key to use for caching
	GetCacheKey() string
}

// Job is the base job that we may use
type Job struct {
	// ID is an identifier for the job
	ID string
	// Method can be one valid HTTP method
	Method string
	// Body is the request's body
	Body []byte
	// URL is the url to sent a request
	URL string
	// Headers is the map of headers to use in HTTP
	Headers map[string]string
	// UrlParams are the url parameters to use in the query string
	UrlParams map[string]string
	// Timeout is the timeout of that job. By timeout we mean the time
	// it takes to finish a single crawl
	Timeout time.Duration
	// Priority is a number indicating the priority. By convention the higher
	// the priority
	Priority int
	// MaxRetries defines the maximum number of retries when a job fails
	MaxRetries int
	// CheckResponse is a function that takes as an input a Response and returns:
	// true: when the response is to be accepted
	// false: when the response is to be rejected
	// By default a response is accepted if status code is 200
	CheckResponse func(resp Response) bool
	// RetryPolicy can be one of:
	// RetryJob: to retry the job untl it's sucessful
	// DiscardJob:for not accepted responses just discard them and do not retry the job
	// RefreshIP: Similar to RetryJob with an importan difference
	// 				Before the job is retried the IP is refreshed.
	RetryPolicy RetryPolicy
	// MaxRetryDelay By default when a job is rejected is retried with an exponential backof
	// for a MaxRetries numbers of time. If the sleep time between the retries is more than
	// MaxRetryDelay then it's capped to that. (Default is 2 seconds)
	MaxRetryDelay time.Duration
	//TakeScreenshot if true takes a screenshot of the page
	TakeScreenshot bool
	Response       Response
}

// GetCacheKey returns the key to use for caching
func (j *Job) GetCacheKey() string {
	urlvals := url.Values{}
	keys := make([]string, 0, len(j.UrlParams))
	for k, _ := range j.UrlParams {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		urlvals.Add(k, j.UrlParams[k])
	}
	u := j.GetURL() + urlvals.Encode()
	toHash := fmt.Sprintf("%s:%s", j.GetMethod(), u)
	if j.GetMethod() == http.MethodPost {
		toHash += string(j.GetBody())
	}
	hashValue := md5.Sum([]byte(toHash))
	cacheKey := hex.EncodeToString(hashValue[:])
	return cacheKey
}

// DoScreenshot used to check if we need a screenshot
// It's here since it's a common use case
func (j *Job) DoScreenshot() bool {
	return j.TakeScreenshot
}

// BrowserActions is the function that will be executed in the browser
// This is the function that will be executed in the browser
// this is a default implementation that will just return the response
// override this function to perform actions in the browser
func (j *Job) BrowserActions(browser playwright.Browser) Response {
	var resp Response
	bctx, err := browser.NewContext(playwright.BrowserNewContextOptions{})
	if err != nil {
		resp.Error = err
		return resp
	}
	defer bctx.Close()
	page, err := bctx.NewPage()
	if err != nil {
		resp.Error = err
		return resp
	}
	defer page.Close()
	if err := page.SetViewportSize(1920, 1057); err != nil {
		resp.Error = err
		return resp
	}
	pageResponse, err := page.Goto(j.GetURL(), playwright.PageGotoOptions{
		WaitUntil: playwright.WaitUntilStateNetworkidle,
	})
	if err != nil {
		resp.Error = err
		return resp
	}
	resp.URL = pageResponse.URL()
	resp.StatusCode = pageResponse.Status()
	resp.Headers = make(http.Header, len(pageResponse.Headers()))
	for k, v := range pageResponse.Headers() {
		resp.Headers.Add(k, v)
	}
	body, err := pageResponse.Body()
	if err != nil {
		resp.Error = err
		return resp
	}
	resp.Body = []byte(body)
	if j.DoScreenshot() {
		screenshot, err := page.Screenshot(playwright.PageScreenshotOptions{
			FullPage: playwright.Bool(true),
		})
		if err != nil {
			resp.Error = err
			return resp
		}
		resp.Screenshot = screenshot
	}
	return resp
}

// String returns the string representation of the job
func (j *Job) String() string {
	return fmt.Sprintf("Job{ID: %s, Method: %s, URL: %s, UrlParams: %v}", j.ID, j.Method, j.URL, j.UrlParams)
}

// Process processes the job
func (j *Job) Process(ctx context.Context, resp Response) (any, []IJob, error) {
	return nil, nil, nil
}

// CheckResponse checks the response of the job
func (j *Job) DoCheckResponse(resp Response) bool {
	if j.CheckResponse == nil {
		return func(resp Response) bool {
			return resp.StatusCode >= 200 && resp.StatusCode < 300
		}(resp)
	}
	return j.CheckResponse(resp)
}

// GetRetryPolicy returns the action to perform on the response
func (j *Job) GetRetryPolicy() RetryPolicy {
	return j.RetryPolicy
}

// GetMaxRetry returns the max retry of the job
func (j *Job) GetMaxRetries() int {
	return j.MaxRetries
}

// GetID returns the unique identifier of the job.
func (j *Job) GetID() string {
	return j.ID
}

// GetMethod returns the http method to use
func (j *Job) GetMethod() string {
	return j.Method
}

// GetBody returns the body of the request
func (j *Job) GetBody() []byte {
	return j.Body
}

// GetURL returns the url to request
func (j *Job) GetURL() string {
	return j.URL
}

// GetHeaders returns the headers to use
func (j *Job) GetHeaders() map[string]string {
	return j.Headers
}

// GetUrlParams returns the url params to use
func (j *Job) GetUrlParams() map[string]string {
	return j.UrlParams
}

// GetTimeout returns the timeout of the job
func (j *Job) GetTimeout() time.Duration {
	return j.Timeout
}

// GetPriority returns the priority of the job
func (j *Job) GetPriority() int {
	return j.Priority
}

// GetRetryDelay returns the delay to wait before retrying
func (j *Job) GetMaxRetryDelay() time.Duration {
	if j.MaxRetryDelay == 0 {
		return DefaultMaxRetryDelay
	}
	return j.MaxRetryDelay
}

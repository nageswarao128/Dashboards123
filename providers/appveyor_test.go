package providers

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/nbedos/citop/cache"
	"github.com/nbedos/citop/utils"
)

func TestParseAppVeyorURL(t *testing.T) {
	u := "https://ci.appveyor.com/project/nbedos/citop/builds/29070120"
	owner, repo, id, err := parseAppVeyorURL(u)
	if err != nil {
		t.Fatal(err)
	}
	if owner != "nbedos" || repo != "citop" || id != 29070120 {
		t.Fail()
	}
}

func TestAppVeyorJob_ToCacheJob(t *testing.T) {
	j := appVeyorJob{
		ID:           "id",
		Name:         "name",
		AllowFailure: true,
		Status:       "success",
		CreatedAt:    "2019-11-23T12:24:26.9181871+00:00",
		StartedAt:    "2019-11-23T12:24:31.8145735+00:00",
		FinishedAt:   "2019-11-23T12:24:34.5646724+00:00",
	}

	expectedJob := cache.Job{
		ID:    "id",
		State: "passed",
		Name:  "name",
		CreatedAt: utils.NullTime{
			Valid: true,
			Time:  time.Date(2019, 11, 23, 12, 24, 26, 918187100, time.UTC),
		},
		StartedAt: utils.NullTime{
			Valid: true,
			Time:  time.Date(2019, 11, 23, 12, 24, 31, 814573500, time.UTC),
		},
		FinishedAt: utils.NullTime{
			Valid: true,
			Time:  time.Date(2019, 11, 23, 12, 24, 34, 564672400, time.UTC),
		},
		Duration: utils.NullDuration{
			Valid:    true,
			Duration: 2750098900 * time.Nanosecond,
		},
		AllowFailure: true,
		WebURL:       "buildURL/job/id",
	}

	job, err := j.toCacheJob(expectedJob.ID, "buildURL")
	if err != nil {
		t.Fatal(err)
	}

	if diff := cmp.Diff(expectedJob, job); len(diff) > 0 {
		t.Fatal(diff)
	}
}

func TestAppVeyorBuild_ToCacheBuild(t *testing.T) {
	repo := cache.Repository{
		Provider: cache.Provider{
			ID:   "id",
			Name: "name",
		},
		ID:    42,
		URL:   "github.com/owner/repo",
		Owner: "owner",
		Name:  "repo",
	}
	b := appVeyorBuild{
		ID:          42,
		Jobs:        nil,
		Number:      42,
		Version:     "1.0.42",
		Message:     "message",
		Branch:      "feature/appveyor",
		IsTag:       false,
		Sha:         "fd4c4ae5a4005e38c66566e2480087072620e9de",
		Author:      "nbedos",
		CommittedAt: "2019-11-23T12:23:09+00:00",
		Status:      "failed",
		CreatedAt:   "2019-11-23T12:24:25.5900258+00:00",
		StartedAt:   "2019-11-23T12:24:31.8145735+00:00",
		FinishedAt:  "2019-11-23T12:24:34.5646724+00:00",
		UpdatedAt:   "2019-11-23T12:24:34.5646724+00:00",
	}

	expectedBuild := cache.Build{
		Repository: &repo,
		ID:         "42",
		Commit: cache.Commit{
			Sha:     "fd4c4ae5a4005e38c66566e2480087072620e9de",
			Message: "message",
			Date: utils.NullTime{
				Valid: true,
				Time:  time.Date(2019, 11, 23, 12, 23, 9, 0, time.UTC),
			},
		},
		Ref:             "feature/appveyor",
		IsTag:           false,
		RepoBuildNumber: "42",
		State:           "failed",
		CreatedAt: utils.NullTime{
			Valid: true,
			Time:  time.Date(2019, 11, 23, 12, 24, 25, 590025800, time.UTC),
		},
		StartedAt: utils.NullTime{
			Valid: true,
			Time:  time.Date(2019, 11, 23, 12, 24, 31, 814573500, time.UTC),
		},
		FinishedAt: utils.NullTime{
			Valid: true,
			Time:  time.Date(2019, 11, 23, 12, 24, 34, 564672400, time.UTC),
		},
		UpdatedAt: time.Date(2019, 11, 23, 12, 24, 34, 564672400, time.UTC),
		Duration: utils.NullDuration{
			Valid:    true,
			Duration: 2750098900 * time.Nanosecond,
		},
		WebURL: "https://ci.appveyor.com/project/owner/repo/builds/42",
		Stages: make(map[int]*cache.Stage),
		Jobs:   make([]*cache.Job, 0),
	}

	build, err := b.toCacheBuild("account", &repo)
	if err != nil {
		t.Fatal(err)
	}

	if diff := cmp.Diff(build, expectedBuild); len(diff) > 0 {
		t.Fatal(diff)
	}
}

func TestCircleCIClient_BuildFromURL(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		filename := ""
		switch {
		case r.Method == "GET" && r.URL.Path == "/api/projects/nbedos/citop/history":
			filename = "appveyor_history_29070120.json"
		case r.Method == "GET" && r.URL.Path == "/api/projects/nbedos/citop/build/1.0.22":
			filename = "appveyor_build_1_0_22.json"
		default:
			w.WriteHeader(404)
			return
		}

		bs, err := ioutil.ReadFile(fmt.Sprintf("test_data/%s", filename))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := fmt.Fprint(w, string(bs)); err != nil {
			t.Fatal(err)
		}
	}))
	defer ts.Close()

	tsu, err := url.Parse(ts.URL)
	if err != nil {
		t.Fatal(err)
	}
	tsu.Path += "/api"
	tsu.RawPath += "/api"

	client := AppVeyorClient{
		url:         *tsu,
		client:      &http.Client{Timeout: 10 * time.Second},
		rateLimiter: time.Tick(time.Millisecond),
		token:       "token",
		provider: cache.Provider{
			ID:   "id",
			Name: "name",
		},
	}

	buildURL := "https://ci.appveyor.com/project/nbedos/citop/builds/29070120"
	build, err := client.BuildFromURL(context.Background(), buildURL)
	if err != nil {
		t.Fatal(err)
	}
	if build.ID != "29070120" {
		t.Fatal()
	}
}

func TestCircleCIClient_Log(t *testing.T) {
	expectedLog := "log\n"
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "GET" && r.URL.Path == "/api/buildjobs/jobId/log":
			if _, err := fmt.Fprint(w, expectedLog); err != nil {
				t.Fatal(err)
			}
		default:
			w.WriteHeader(404)
			return
		}
	}))
	defer ts.Close()

	tsu, err := url.Parse(ts.URL)
	if err != nil {
		t.Fatal(err)
	}
	tsu.Path += "/api"
	tsu.RawPath += "/api"

	client := AppVeyorClient{
		url:         *tsu,
		client:      &http.Client{Timeout: 10 * time.Second},
		rateLimiter: time.Tick(time.Millisecond),
		token:       "token",
		provider: cache.Provider{
			ID:   "id",
			Name: "name",
		},
	}

	log, err := client.Log(context.Background(), cache.Repository{}, "jobId")
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(log, expectedLog); len(diff) > 0 {
		t.Fatal(diff)
	}
}

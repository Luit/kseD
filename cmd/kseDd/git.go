package main

import (
	"encoding/base64"
	"errors"
	"expvar"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/google/go-github/github"
)

const (
	owner = "Luit"
	repo  = "kseD-config"
	ref   = "heads/master"
)

var (
	client   = github.NewClient((&githubTransport{}).Client())
	gitError = errors.New("unexpected result from git")

	githubClientId     = os.Getenv("GITHUB_CLIENT_ID")
	githubClientSecret = os.Getenv("GITHUB_CLIENT_SECRET")
)

var (
	rateLimitRemaining = expvar.NewInt("ratelimit_remaining")
	rateLimitLimit     = expvar.NewInt("ratelimit_limt")
	rateLimitReset     = expvar.NewInt("ratelimit_reset")
	rateLimitTime      = expvar.NewInt("ratelimit_time")
)

type githubTransport struct{}

func (t *githubTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	transport := http.DefaultTransport
	if githubClientId != "" && githubClientSecret != "" {
		transport = &github.UnauthenticatedRateLimitedTransport{
			ClientID:     githubClientId,
			ClientSecret: githubClientSecret,
		}
	}
	resp, err := transport.RoundTrip(req)
	if err != nil {
		return resp, err
	}
	remaining, err := strconv.ParseInt(resp.Header.Get("X-Ratelimit-Remaining"), 10, 0)
	if err == nil {
		rateLimitRemaining.Set(remaining)
	}
	limit, err := strconv.ParseInt(resp.Header.Get("X-Ratelimit-Limit"), 10, 0)
	if err == nil {
		rateLimitLimit.Set(limit)
	}
	now := time.Now().Unix()
	reset, err := strconv.ParseInt(resp.Header.Get("X-Ratelimit-Reset"), 10, 0)
	if err == nil {
		rateLimitReset.Set(reset - now)
		rateLimitTime.Set(now)
	}
	return resp, err
}

func (t *githubTransport) Client() *http.Client {
	return &http.Client{Transport: t}
}

func currentMaster() (commitSha string, err error) {
	r, _, err := client.Git.GetRef(owner, repo, ref)
	if err != nil {
		return "", err
	}
	if r == nil || r.Object == nil || r.Object.Type == nil || r.Object.SHA == nil {
		return "", gitError
	}
	if *r.Object.Type != "commit" {
		return "", gitError
	}
	return *r.Object.SHA, nil
}

func getTree(commitSha string) (treeSha string, err error) {
	commit, _, err := client.Git.GetCommit(owner, repo, commitSha)
	if err != nil {
		return "", err
	}
	if commit == nil || commit.Tree == nil || commit.Tree.SHA == nil {
		return "", gitError
	}
	return *commit.Tree.SHA, nil
}

func getBlob(treeSha, path string) (blobSha string, err error) {
	tree, _, err := client.Git.GetTree(owner, repo, treeSha, false)
	if err != nil {
		return "", err
	}
	for _, e := range tree.Entries {
		if e.Path == nil || e.SHA == nil {
			continue
		}
		if *e.Path == "kseD.json" {
			blobSha = *e.SHA
		}
	}
	if blobSha == "" {
		return "", gitError
	}
	return blobSha, nil
}

func blobReader(blobSha string) (io.Reader, error) {
	blob, _, err := client.Git.GetBlob(owner, repo, blobSha)
	if err != nil {
		return nil, gitError
	}
	if blob == nil || blob.Content == nil || blob.Encoding == nil {
		return nil, gitError
	}
	if *blob.Encoding != "utf-8" && *blob.Encoding != "base64" {
		return nil, gitError
	}
	r := strings.NewReader(*blob.Content)
	if *blob.Encoding == "base64" {
		return base64.NewDecoder(base64.StdEncoding, r), nil
	}
	return r, nil
}

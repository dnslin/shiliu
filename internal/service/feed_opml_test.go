package service_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	v1 "shiliu/api/v1"
)

func TestFeedServiceImportOPMLCreatesFeedsThroughExistingFetchPipeline(t *testing.T) {
	ctx := context.Background()
	feedURL := "https://example.com/articles.xml"
	fetcher := newFixtureFetcher(t, map[string]string{
		feedURL: "rss_initial.xml",
	})
	svc, feedRepo, contentRepo := newFeedServiceHarness(t, fetcher)

	result, err := svc.ImportOPML(ctx, &v1.ImportOPMLRequest{OPML: `<?xml version="1.0" encoding="UTF-8"?>
<opml version="2.0">
  <body>
    <outline text="Reading">
      <outline text="Articles" title="Articles" type="rss" xmlUrl=" HTTPS://EXAMPLE.com:443/articles.xml#folder "/>
    </outline>
  </body>
</opml>`})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 1, result.Total)
	assert.Equal(t, 1, result.Success)
	assert.Equal(t, 0, result.Duplicate)
	assert.Equal(t, 0, result.Failed)
	assert.Equal(t, []string{feedURL}, fetcher.requests)

	feed, err := feedRepo.GetByURL(ctx, feedURL)
	require.NoError(t, err)
	require.NotNil(t, feed)
	assert.Nil(t, feed.FolderID)

	items, err := contentRepo.ListByFeedID(ctx, feed.Id, 10)
	require.NoError(t, err)
	assert.NotEmpty(t, items)
}

func TestFeedServiceImportOPMLReportsMixedSuccessDuplicateAndFailedCounts(t *testing.T) {
	ctx := context.Background()
	existingURL := "https://example.com/existing.xml"
	newURL := "https://example.com/new.xml"
	notFeedURL := "https://example.com/not-feed.xml"
	fetcher := newFixtureFetcher(t, map[string]string{
		existingURL: "rss_initial.xml",
		newURL:      "rss_incremental.xml",
		notFeedURL:  "not_feed.xml",
	})
	svc, feedRepo, _ := newFeedServiceHarness(t, fetcher)

	created, err := svc.CreateFeed(ctx, &v1.CreateFeedRequest{FeedURL: existingURL})
	require.NoError(t, err)
	require.NotZero(t, created.Id)
	fetcher.requests = nil

	result, err := svc.ImportOPML(ctx, &v1.ImportOPMLRequest{OPML: `<?xml version="1.0" encoding="UTF-8"?>
<opml version="2.0">
  <body>
    <outline text="Existing" xmlUrl=" HTTPS://EXAMPLE.com:443/existing.xml#ignored "/>
    <outline text="New" xmlURL="https://example.com/new.xml"/>
    <outline text="Duplicate in payload" xmlUrl="https://example.com/new.xml#again"/>
    <outline text="Not a feed" url="https://example.com/not-feed.xml"/>
    <outline text="Bad URL" xmlUrl="example.com/no-scheme.xml"/>
  </body>
</opml>`})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 5, result.Total)
	assert.Equal(t, 1, result.Success)
	assert.Equal(t, 2, result.Duplicate)
	assert.Equal(t, 2, result.Failed)
	assert.Equal(t, []string{newURL, notFeedURL}, fetcher.requests)

	feeds, err := feedRepo.List(ctx)
	require.NoError(t, err)
	assert.Len(t, feeds, 2)
}

func TestFeedServiceImportOPMLRejectsInvalidOPML(t *testing.T) {
	ctx := context.Background()
	svc, _, _ := newFeedServiceHarness(t, newFixtureFetcher(t, nil))

	for _, tt := range []struct {
		name string
		opml string
	}{
		{name: "empty", opml: ""},
		{name: "malformed xml", opml: `<opml><body><outline xmlUrl="https://example.com/feed.xml"></body></opml>`},
		{name: "no feed urls", opml: `<opml version="2.0"><body><outline text="Folder"><outline text="Child"/></outline></body></opml>`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			result, err := svc.ImportOPML(ctx, &v1.ImportOPMLRequest{OPML: tt.opml})

			assert.Nil(t, result)
			require.Error(t, err)
			assert.True(t, errors.Is(err, v1.ErrOPMLInvalid), "got %v", err)
		})
	}
}

package service_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zapcore"

	v1 "shiliu/api/v1"
	"shiliu/internal/model"
	"shiliu/internal/repository"
	"shiliu/internal/service"
)

func TestTagService_RenameTagRespondsWithRequestedNameWithoutReadAfterWrite(t *testing.T) {
	logger, _ := newObservedLogger(zapcore.InfoLevel)
	tagRepo := &tagRepositorySpy{getByIDResult: &model.Tag{Id: 7, Name: "raced"}}
	svc := service.NewTagService(service.NewService(nil, logger, nil, nil), tagRepo, &tagContentRepositorySpy{})

	result, err := svc.RenameTag(context.Background(), 7, &v1.RenameTagRequest{Name: " work "})

	require.NoError(t, err)
	assert.Equal(t, uint(7), result.Id)
	assert.Equal(t, "work", result.Name)
	assert.Zero(t, tagRepo.getByIDCalls, "rename response must not read mutable state after the write commits")
}

func TestTagService_AssignAndRemoveTagsDelegateTagExistenceToRepository(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(service.TagService, context.Context, uint, *v1.AssignContentItemTagsRequest) error
		calls  func(*tagContentRepositorySpy) int
	}{
		{
			name:   "assign",
			change: service.TagService.AssignContentItemTags,
			calls:  func(repo *tagContentRepositorySpy) int { return repo.assignCalls },
		},
		{
			name:   "remove",
			change: service.TagService.RemoveContentItemTags,
			calls:  func(repo *tagContentRepositorySpy) int { return repo.removeCalls },
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			logger, _ := newObservedLogger(zapcore.InfoLevel)
			tagRepo := &tagRepositorySpy{}
			contentRepo := &tagContentRepositorySpy{}
			svc := service.NewTagService(service.NewService(nil, logger, nil, nil), tagRepo, contentRepo)

			err := tc.change(svc, context.Background(), 42, &v1.AssignContentItemTagsRequest{TagIDs: []uint{3, 5, 3}})

			require.NoError(t, err)
			assert.Zero(t, tagRepo.getByIDCalls, "tag existence belongs to repository batch validation")
			assert.Equal(t, 1, tc.calls(contentRepo))
			assert.Equal(t, []uint{3, 5}, contentRepo.lastTagIDs)
		})
	}
}

type tagRepositorySpy struct {
	getByIDCalls  int
	getByIDResult *model.Tag
	renameID      uint
	renameName    string
}

func (r *tagRepositorySpy) Create(context.Context, *model.Tag) error { return nil }

func (r *tagRepositorySpy) GetByID(_ context.Context, id uint) (*model.Tag, error) {
	r.getByIDCalls++
	if r.getByIDResult != nil {
		return r.getByIDResult, nil
	}
	return &model.Tag{Id: id, Name: "tag"}, nil
}

func (r *tagRepositorySpy) List(context.Context) ([]*model.Tag, error) { return nil, nil }

func (r *tagRepositorySpy) Rename(_ context.Context, id uint, name string) error {
	r.renameID = id
	r.renameName = name
	return nil
}

func (r *tagRepositorySpy) Delete(context.Context, uint) error { return nil }

type tagContentRepositorySpy struct {
	assignCalls int
	removeCalls int
	lastTagIDs  []uint
}

func (r *tagContentRepositorySpy) Create(context.Context, *model.ContentItem) error { return nil }

func (r *tagContentRepositorySpy) GetByID(_ context.Context, id uint) (*model.ContentItem, error) {
	return &model.ContentItem{Id: id}, nil
}

func (r *tagContentRepositorySpy) GetExportDataByID(context.Context, uint) (*repository.ContentItemExportData, error) {
	return nil, v1.ErrNotFound
}

func (r *tagContentRepositorySpy) GetByFeedAndDedupeKey(context.Context, uint, string) (*model.ContentItem, error) {
	return nil, nil
}

func (r *tagContentRepositorySpy) List(context.Context, repository.ContentItemListFilter, int, int) ([]*model.ContentItem, int64, error) {
	return nil, 0, nil
}

func (r *tagContentRepositorySpy) ListByFeedID(context.Context, uint, int) ([]*model.ContentItem, error) {
	return nil, nil
}

func (r *tagContentRepositorySpy) ListAutoSummaryCandidates(context.Context, repository.AutoSummaryCandidateFilter, int) ([]*model.ContentItem, error) {
	return nil, nil
}

func (r *tagContentRepositorySpy) AssignTags(_ context.Context, _ uint, tagIDs []uint) error {
	r.assignCalls++
	r.lastTagIDs = append([]uint(nil), tagIDs...)
	return nil
}

func (r *tagContentRepositorySpy) RemoveTags(_ context.Context, _ uint, tagIDs []uint) error {
	r.removeCalls++
	r.lastTagIDs = append([]uint(nil), tagIDs...)
	return nil
}

func (r *tagContentRepositorySpy) UpdateProcessingStatus(context.Context, uint, model.ContentItemProcessingStatus) error {
	return nil
}

func (r *tagContentRepositorySpy) UpdateMark(context.Context, uint, model.ContentItemMark, bool) error {
	return nil
}

func (r *tagContentRepositorySpy) UpdateAudioProgress(context.Context, uint, int) error { return nil }

func (r *tagContentRepositorySpy) UpdateSearchText(context.Context, uint, string, string) error {
	return nil
}

func (r *tagContentRepositorySpy) UpdateAISummarySearchText(context.Context, uint, string) error {
	return nil
}

func (r *tagContentRepositorySpy) ClaimAISummary(context.Context, uint, []model.AISummaryStatus) error {
	return nil
}

func (r *tagContentRepositorySpy) UpdateAISummary(context.Context, uint, model.AISummaryStatus, string, *time.Time, string) error {
	return nil
}

package service

import (
	"context"
	"errors"
	"strings"

	"gorm.io/gorm"

	v1 "shiliu/api/v1"
	"shiliu/internal/model"
	"shiliu/internal/repository"
)

type TagService interface {
	CreateTag(ctx context.Context, req *v1.CreateTagRequest) (*v1.TagResponseData, error)
	ListTags(ctx context.Context) (*v1.ListTagsResponseData, error)
	RenameTag(ctx context.Context, id uint, req *v1.RenameTagRequest) (*v1.TagResponseData, error)
	DeleteTag(ctx context.Context, id uint) error
	AssignContentItemTags(ctx context.Context, itemID uint, req *v1.AssignContentItemTagsRequest) error
	RemoveContentItemTags(ctx context.Context, itemID uint, req *v1.AssignContentItemTagsRequest) error
}

func NewTagService(service *Service, tagRepo repository.TagRepository, contentRepo repository.ContentItemRepository) TagService {
	return &tagService{Service: service, tagRepo: tagRepo, contentRepo: contentRepo}
}

type tagService struct {
	tagRepo     repository.TagRepository
	contentRepo repository.ContentItemRepository
	*Service
}

func (s *tagService) CreateTag(ctx context.Context, req *v1.CreateTagRequest) (*v1.TagResponseData, error) {
	name, err := tagNameFromRequest(req)
	if err != nil {
		return nil, err
	}
	tag := &model.Tag{Name: name}
	if err := s.tagRepo.Create(ctx, tag); err != nil {
		return nil, mapTagWriteError(err)
	}
	response := tagResponseFromModel(tag)
	return &response, nil
}

func (s *tagService) ListTags(ctx context.Context) (*v1.ListTagsResponseData, error) {
	tags, err := s.tagRepo.List(ctx)
	if err != nil {
		return nil, err
	}
	response := &v1.ListTagsResponseData{
		Total: len(tags),
		Items: make([]v1.TagResponseData, 0, len(tags)),
	}
	for _, tag := range tags {
		response.Items = append(response.Items, tagResponseFromModel(tag))
	}
	return response, nil
}

func (s *tagService) RenameTag(ctx context.Context, id uint, req *v1.RenameTagRequest) (*v1.TagResponseData, error) {
	if id == 0 {
		return nil, v1.ErrBadRequest
	}
	if req == nil {
		return nil, v1.ErrBadRequest
	}
	name, err := normalizeTagName(req.Name)
	if err != nil {
		return nil, err
	}
	if err := s.tagRepo.Rename(ctx, id, name); err != nil {
		return nil, mapTagWriteError(err)
	}
	tag, err := s.tagRepo.GetByID(ctx, id)
	if err != nil {
		return nil, mapTagReadError(err)
	}
	response := tagResponseFromModel(tag)
	return &response, nil
}

func (s *tagService) DeleteTag(ctx context.Context, id uint) error {
	if id == 0 {
		return v1.ErrBadRequest
	}
	if err := s.tagRepo.Delete(ctx, id); err != nil {
		return mapTagReadError(err)
	}
	return nil
}

func (s *tagService) AssignContentItemTags(ctx context.Context, itemID uint, req *v1.AssignContentItemTagsRequest) error {
	return s.changeContentItemTags(ctx, itemID, req, s.contentRepo.AssignTags)
}

func (s *tagService) RemoveContentItemTags(ctx context.Context, itemID uint, req *v1.AssignContentItemTagsRequest) error {
	return s.changeContentItemTags(ctx, itemID, req, s.contentRepo.RemoveTags)
}

func (s *tagService) changeContentItemTags(ctx context.Context, itemID uint, req *v1.AssignContentItemTagsRequest, change func(context.Context, uint, []uint) error) error {
	if itemID == 0 {
		return v1.ErrBadRequest
	}
	tagIDs, err := tagIDsFromRequest(req)
	if err != nil {
		return err
	}
	return s.tm.Transaction(ctx, func(ctx context.Context) error {
		if _, err := s.contentRepo.GetByID(ctx, itemID); err != nil {
			if errors.Is(err, v1.ErrNotFound) || errors.Is(err, v1.ErrContentItemNotFound) {
				return v1.ErrContentItemNotFound
			}
			return err
		}
		for _, tagID := range tagIDs {
			if _, err := s.tagRepo.GetByID(ctx, tagID); err != nil {
				return mapTagReadError(err)
			}
		}
		if err := change(ctx, itemID, tagIDs); err != nil {
			if errors.Is(err, v1.ErrNotFound) {
				return v1.ErrContentItemNotFound
			}
			return err
		}
		return nil
	})
}

func tagNameFromRequest(req *v1.CreateTagRequest) (string, error) {
	if req == nil {
		return "", v1.ErrBadRequest
	}
	return normalizeTagName(req.Name)
}

func normalizeTagName(raw string) (string, error) {
	name := strings.TrimSpace(raw)
	if name == "" {
		return "", v1.ErrBadRequest
	}
	return name, nil
}

func tagIDsFromRequest(req *v1.AssignContentItemTagsRequest) ([]uint, error) {
	if req == nil || len(req.TagIDs) == 0 {
		return nil, v1.ErrBadRequest
	}
	seen := make(map[uint]struct{}, len(req.TagIDs))
	ids := make([]uint, 0, len(req.TagIDs))
	for _, id := range req.TagIDs {
		if id == 0 {
			return nil, v1.ErrBadRequest
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	return ids, nil
}

func mapTagWriteError(err error) error {
	switch {
	case errors.Is(err, gorm.ErrDuplicatedKey):
		return v1.ErrTagAlreadyExists
	case errors.Is(err, v1.ErrNotFound):
		return v1.ErrTagNotFound
	default:
		return err
	}
}

func mapTagReadError(err error) error {
	if errors.Is(err, v1.ErrNotFound) {
		return v1.ErrTagNotFound
	}
	return err
}

func tagResponseFromModel(tag *model.Tag) v1.TagResponseData {
	if tag == nil {
		return v1.TagResponseData{}
	}
	return v1.TagResponseData{Id: tag.Id, Name: tag.Name}
}

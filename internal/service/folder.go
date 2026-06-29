package service

import (
	"context"
	"errors"
	"strings"

	v1 "shiliu/api/v1"
	"shiliu/internal/model"
	"shiliu/internal/repository"

	"gorm.io/gorm"
)

type FolderService interface {
	CreateFolder(ctx context.Context, req *v1.CreateFolderRequest) (*v1.FolderResponseData, error)
	ListFolders(ctx context.Context) (*v1.ListFoldersResponseData, error)
	RenameFolder(ctx context.Context, id uint, req *v1.RenameFolderRequest) (*v1.FolderResponseData, error)
	DeleteFolder(ctx context.Context, id uint) error
	AssignFeedFolder(ctx context.Context, feedID uint, req *v1.AssignFeedFolderRequest) error
}

func NewFolderService(service *Service, folderRepo repository.FolderRepository, feedRepo repository.FeedRepository) FolderService {
	return &folderService{Service: service, folderRepo: folderRepo, feedRepo: feedRepo}
}

type folderService struct {
	folderRepo repository.FolderRepository
	feedRepo   repository.FeedRepository
	*Service
}

func (s *folderService) CreateFolder(ctx context.Context, req *v1.CreateFolderRequest) (*v1.FolderResponseData, error) {
	name, err := folderNameFromRequest(req)
	if err != nil {
		return nil, err
	}
	folder := &model.Folder{Name: name}
	if err := s.folderRepo.Create(ctx, folder); err != nil {
		return nil, mapFolderWriteError(err)
	}
	response := folderResponseFromModel(folder)
	return &response, nil
}

func (s *folderService) ListFolders(ctx context.Context) (*v1.ListFoldersResponseData, error) {
	folders, err := s.folderRepo.List(ctx)
	if err != nil {
		return nil, err
	}
	response := &v1.ListFoldersResponseData{
		Total: len(folders),
		Items: make([]v1.FolderResponseData, 0, len(folders)),
	}
	for _, folder := range folders {
		response.Items = append(response.Items, folderResponseFromModel(folder))
	}
	return response, nil
}

func (s *folderService) RenameFolder(ctx context.Context, id uint, req *v1.RenameFolderRequest) (*v1.FolderResponseData, error) {
	if id == 0 || req == nil {
		return nil, v1.ErrBadRequest
	}
	name, err := normalizeFolderName(req.Name)
	if err != nil {
		return nil, err
	}
	if err := s.folderRepo.Rename(ctx, id, name); err != nil {
		return nil, mapFolderWriteError(err)
	}
	return &v1.FolderResponseData{Id: id, Name: name}, nil
}

func (s *folderService) DeleteFolder(ctx context.Context, id uint) error {
	if id == 0 {
		return v1.ErrBadRequest
	}
	if err := s.folderRepo.Delete(ctx, id); err != nil {
		return mapFolderReadError(err)
	}
	return nil
}

func (s *folderService) AssignFeedFolder(ctx context.Context, feedID uint, req *v1.AssignFeedFolderRequest) error {
	if feedID == 0 || req == nil || !req.HasFolderID() {
		return v1.ErrBadRequest
	}
	return s.transaction(ctx, func(txCtx context.Context) error {
		if req.FolderID != nil {
			if *req.FolderID == 0 {
				return v1.ErrBadRequest
			}
			if _, err := s.folderRepo.GetByID(txCtx, *req.FolderID); err != nil {
				return mapFolderReadError(err)
			}
		}
		return s.feedRepo.AssignFolder(txCtx, feedID, req.FolderID)
	})
}

func (s *folderService) transaction(ctx context.Context, fn func(context.Context) error) error {
	if s.Service == nil || s.tm == nil {
		return fn(ctx)
	}
	return s.tm.Transaction(ctx, fn)
}

func folderNameFromRequest(req *v1.CreateFolderRequest) (string, error) {
	if req == nil {
		return "", v1.ErrBadRequest
	}
	return normalizeFolderName(req.Name)
}

func normalizeFolderName(raw string) (string, error) {
	name := strings.TrimSpace(raw)
	if name == "" {
		return "", v1.ErrBadRequest
	}
	return name, nil
}

func mapFolderWriteError(err error) error {
	switch {
	case errors.Is(err, gorm.ErrDuplicatedKey):
		return v1.ErrFolderAlreadyExists
	case errors.Is(err, v1.ErrNotFound):
		return v1.ErrFolderNotFound
	default:
		return err
	}
}

func mapFolderReadError(err error) error {
	if errors.Is(err, v1.ErrNotFound) {
		return v1.ErrFolderNotFound
	}
	return err
}

func folderResponseFromModel(folder *model.Folder) v1.FolderResponseData {
	if folder == nil {
		return v1.FolderResponseData{}
	}
	return v1.FolderResponseData{Id: folder.Id, Name: folder.Name}
}

package repository

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	v1 "shiliu/api/v1"
	"shiliu/internal/model"

	"gorm.io/gorm"
)

type ContentItemListFilter struct {
	ContentType      *model.ContentItemType
	ProcessingStatus *model.ContentItemProcessingStatus
	Mark             *model.ContentItemMark
	FeedID           *uint
	TagID            *uint
	FolderID         *uint
	Keyword          string
}

type AutoSummaryCandidateFilter struct {
	EnabledAt        time.Time
	ContentTypeScope model.AutoSummaryContentTypeScope
}

type ContentItemExportData struct {
	ContentItemID        uint
	Title                string
	ContentType          model.ContentItemType
	AvailableText        string
	PublishedAt          *time.Time
	AISummaryStatus      model.AISummaryStatus
	AISummaryMarkdown    string
	AISummaryGeneratedAt *time.Time
	AISummaryError       string
	FeedID               uint
	FeedTitle            string
	FeedURL              string
	FolderName           *string
	TagNames             []string
}

type ContentItemRepository interface {
	Create(ctx context.Context, item *model.ContentItem) error
	GetByID(ctx context.Context, id uint) (*model.ContentItem, error)
	GetExportDataByID(ctx context.Context, id uint) (*ContentItemExportData, error)
	GetByFeedAndDedupeKey(ctx context.Context, feedID uint, dedupeKey string) (*model.ContentItem, error)
	List(ctx context.Context, filter ContentItemListFilter, limit int, offset int) ([]*model.ContentItem, int64, error)
	ListByFeedID(ctx context.Context, feedID uint, limit int) ([]*model.ContentItem, error)
	ListAutoSummaryCandidates(ctx context.Context, filter AutoSummaryCandidateFilter, limit int) ([]*model.ContentItem, error)
	AssignTags(ctx context.Context, itemID uint, tagIDs []uint) error
	RemoveTags(ctx context.Context, itemID uint, tagIDs []uint) error
	UpdateProcessingStatus(ctx context.Context, itemID uint, status model.ContentItemProcessingStatus) error
	UpdateMark(ctx context.Context, itemID uint, mark model.ContentItemMark, marked bool) error
	UpdateAudioProgress(ctx context.Context, itemID uint, progressSeconds int) error
	UpdateSearchText(ctx context.Context, itemID uint, title string, availableText string) error
	UpdateAISummarySearchText(ctx context.Context, itemID uint, markdown string) error
	ClaimAISummary(ctx context.Context, itemID uint, allowedStatuses []model.AISummaryStatus) error
	UpdateAISummary(ctx context.Context, itemID uint, status model.AISummaryStatus, markdown string, generatedAt *time.Time, summaryError string) error
}

func NewContentItemRepository(r *Repository) ContentItemRepository {
	return &contentItemRepository{Repository: r}
}

type contentItemRepository struct {
	*Repository
}

func (r *contentItemRepository) Create(ctx context.Context, item *model.ContentItem) error {
	if item.FetchedAt.IsZero() {
		item.FetchedAt = time.Now().UTC()
	}
	if item.ProcessingStatus == "" {
		item.ProcessingStatus = model.ContentItemProcessingStatusUnprocessed
	}
	if item.AISummaryStatus == "" {
		item.AISummaryStatus = model.AISummaryStatusNone
	}
	return r.DB(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(item).Error; err != nil {
			return err
		}
		return upsertContentItemSearchIndex(tx, item.Id)
	})
}

func (r *contentItemRepository) GetByID(ctx context.Context, id uint) (*model.ContentItem, error) {
	var item model.ContentItem
	if err := r.DB(ctx).Where("id = ?", id).First(&item).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, v1.ErrNotFound
		}
		return nil, err
	}
	return &item, nil
}

func (r *contentItemRepository) GetExportDataByID(ctx context.Context, id uint) (*ContentItemExportData, error) {
	if id == 0 {
		return nil, v1.ErrBadRequest
	}

	var data ContentItemExportData
	err := r.DB(ctx).Table("content_items").
		Select(`content_items.id AS content_item_id,
			content_items.title,
			content_items.type AS content_type,
			content_items.available_text,
			content_items.published_at,
			content_items.ai_summary_status,
			content_items.ai_summary_markdown,
			content_items.ai_summary_generated_at,
			content_items.ai_summary_error,
			feeds.id AS feed_id,
			feeds.title AS feed_title,
			feeds.feed_url,
			folders.name AS folder_name`).
		Joins("JOIN feeds ON feeds.id = content_items.feed_id").
		Joins("LEFT JOIN folders ON folders.id = feeds.folder_id").
		Where("content_items.id = ?", id).
		Take(&data).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, v1.ErrNotFound
		}
		return nil, err
	}

	var tagNames []string
	if err := r.DB(ctx).Table("content_item_tags").
		Select("tags.name").
		Joins("JOIN tags ON tags.id = content_item_tags.tag_id").
		Where("content_item_tags.content_item_id = ?", id).
		Order("tags.name ASC, tags.id ASC").
		Pluck("tags.name", &tagNames).Error; err != nil {
		return nil, err
	}
	data.TagNames = tagNames
	return &data, nil
}

func (r *contentItemRepository) GetByFeedAndDedupeKey(ctx context.Context, feedID uint, dedupeKey string) (*model.ContentItem, error) {
	var item model.ContentItem
	if err := r.DB(ctx).Where("feed_id = ? AND dedupe_key = ?", feedID, dedupeKey).First(&item).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &item, nil
}

type contentItemSearchIndexValues struct {
	ItemID            uint
	Title             string
	FeedTitle         string
	AvailableText     string
	AISummaryMarkdown string
}

func upsertContentItemSearchIndex(tx *gorm.DB, itemID uint) error {
	if itemID == 0 {
		return v1.ErrBadRequest
	}
	var values contentItemSearchIndexValues
	err := tx.Table("content_items").
		Select("content_items.id AS item_id, content_items.title, feeds.title AS feed_title, content_items.available_text, content_items.ai_summary_markdown").
		Joins("JOIN feeds ON feeds.id = content_items.feed_id").
		Where("content_items.id = ?", itemID).
		Take(&values).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return v1.ErrNotFound
		}
		return err
	}
	if err := tx.Exec(`DELETE FROM content_item_search_index WHERE rowid = ?`, itemID).Error; err != nil {
		return err
	}
	return tx.Exec(
		`INSERT INTO content_item_search_index (rowid, title, feed_title, available_text, ai_summary_markdown) VALUES (?, ?, ?, ?, ?)`,
		values.ItemID,
		values.Title,
		values.FeedTitle,
		values.AvailableText,
		values.AISummaryMarkdown,
	).Error
}

func syncContentItemSearchIndex(tx *gorm.DB, itemID uint) error {
	return upsertContentItemSearchIndex(tx, itemID)
}

func (r *contentItemRepository) UpdateSearchText(ctx context.Context, itemID uint, title string, availableText string) error {
	if itemID == 0 {
		return v1.ErrBadRequest
	}
	return r.DB(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&model.ContentItem{}).
			Where("id = ?", itemID).
			Updates(map[string]interface{}{
				"title":          title,
				"available_text": availableText,
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return v1.ErrNotFound
		}
		return syncContentItemSearchIndex(tx, itemID)
	})
}

func (r *contentItemRepository) UpdateAISummarySearchText(ctx context.Context, itemID uint, markdown string) error {
	if itemID == 0 {
		return v1.ErrBadRequest
	}
	return r.DB(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&model.ContentItem{}).
			Where("id = ?", itemID).
			Update("ai_summary_markdown", markdown)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return v1.ErrNotFound
		}
		return syncContentItemSearchIndex(tx, itemID)
	})
}

func (r *contentItemRepository) ClaimAISummary(ctx context.Context, itemID uint, allowedStatuses []model.AISummaryStatus) error {
	if len(allowedStatuses) == 0 {
		return v1.ErrBadRequest
	}
	return r.updateAISummary(ctx, itemID, model.AISummaryStatusPending, "", nil, "", allowedStatuses)
}

func (r *contentItemRepository) UpdateAISummary(ctx context.Context, itemID uint, status model.AISummaryStatus, markdown string, generatedAt *time.Time, summaryError string) error {
	var allowedStatuses []model.AISummaryStatus
	if status == model.AISummaryStatusPending {
		allowedStatuses = []model.AISummaryStatus{
			model.AISummaryStatusNone,
			model.AISummaryStatusFailed,
			model.AISummaryStatusSuccess,
		}
	}
	return r.updateAISummary(ctx, itemID, status, markdown, generatedAt, summaryError, allowedStatuses)
}

func (r *contentItemRepository) updateAISummary(ctx context.Context, itemID uint, status model.AISummaryStatus, markdown string, generatedAt *time.Time, summaryError string, allowedStatuses []model.AISummaryStatus) error {
	if itemID == 0 || !validAISummaryStatus(status) {
		return v1.ErrBadRequest
	}
	for _, allowedStatus := range allowedStatuses {
		if !validAISummaryStatus(allowedStatus) {
			return v1.ErrBadRequest
		}
	}
	return r.DB(ctx).Transaction(func(tx *gorm.DB) error {
		query := tx.Model(&model.ContentItem{}).Where("id = ?", itemID)
		if len(allowedStatuses) > 0 {
			query = query.Where("ai_summary_status IN ?", allowedStatuses)
		}
		result := query.Updates(map[string]interface{}{
			"ai_summary_status":       status,
			"ai_summary_markdown":     markdown,
			"ai_summary_generated_at": generatedAt,
			"ai_summary_error":        summaryError,
		})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			if status == model.AISummaryStatusPending {
				return aiSummaryClaimConflictError(tx, itemID)
			}
			return v1.ErrNotFound
		}
		return syncContentItemSearchIndex(tx, itemID)
	})
}

func aiSummaryClaimConflictError(tx *gorm.DB, itemID uint) error {
	var item model.ContentItem
	if err := tx.Select("ai_summary_status").Where("id = ?", itemID).Take(&item).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return v1.ErrNotFound
		}
		return err
	}
	switch item.AISummaryStatus {
	case model.AISummaryStatusInsufficientText:
		return v1.ErrAIInsufficientText
	case model.AISummaryStatusPending:
		return v1.ErrAISummaryInProgress
	default:
		return v1.ErrAISummaryInProgress
	}
}

func validAISummaryStatus(status model.AISummaryStatus) bool {
	switch status {
	case model.AISummaryStatusNone, model.AISummaryStatusPending, model.AISummaryStatusSuccess, model.AISummaryStatusFailed, model.AISummaryStatusInsufficientText:
		return true
	default:
		return false
	}
}

const contentItemListSelect = "content_items.id, content_items.feed_id, content_items.type, content_items.title, content_items.available_text, content_items.published_at, content_items.fetched_at, content_items.processing_status, content_items.marked_later, content_items.favorited, content_items.audio_progress_seconds"
const contentItemListOrder = "COALESCE(content_items.published_at, content_items.fetched_at) DESC, content_items.id DESC"
const contentItemSearchListOrder = "bm25(content_item_search_index), " + contentItemListOrder

const contentItemSearchMaxKeywordRunes = 128

func (r *contentItemRepository) List(ctx context.Context, filter ContentItemListFilter, limit int, offset int) ([]*model.ContentItem, int64, error) {
	if limit < 0 || offset < 0 {
		return nil, 0, v1.ErrBadRequest
	}
	query, err := applyContentItemListFilter(r.DB(ctx).Model(&model.ContentItem{}), filter)
	if err != nil {
		return nil, 0, err
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	order := contentItemListOrder
	if contentItemSearchUsesFTS(filter.Keyword) {
		order = contentItemSearchListOrder
	}
	query = query.Select(contentItemListSelect).Order(order)
	if limit > 0 {
		query = query.Limit(limit)
	}
	if offset > 0 {
		query = query.Offset(offset)
	}

	var items []*model.ContentItem
	if err := query.Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *contentItemRepository) ListByFeedID(ctx context.Context, feedID uint, limit int) ([]*model.ContentItem, error) {
	if feedID == 0 {
		return nil, v1.ErrBadRequest
	}
	query := r.DB(ctx).Where("feed_id = ?", feedID).Order(contentItemListOrder)
	if limit > 0 {
		query = query.Limit(limit)
	}
	var items []*model.ContentItem
	if err := query.Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *contentItemRepository) ListAutoSummaryCandidates(ctx context.Context, filter AutoSummaryCandidateFilter, limit int) ([]*model.ContentItem, error) {
	if limit <= 0 || filter.EnabledAt.IsZero() || !validAutoSummaryContentTypeScope(filter.ContentTypeScope) {
		return nil, v1.ErrBadRequest
	}
	query := r.DB(ctx).
		Where("created_at >= ?", filter.EnabledAt).
		Where("ai_summary_status = ?", model.AISummaryStatusNone)
	switch filter.ContentTypeScope {
	case model.AutoSummaryContentTypeScopeText:
		query = query.Where("type = ?", model.ContentItemTypeText)
	case model.AutoSummaryContentTypeScopeAudio:
		query = query.Where("type = ?", model.ContentItemTypeAudio)
	case model.AutoSummaryContentTypeScopeAll:
		query = query.Where("type IN ?", []model.ContentItemType{model.ContentItemTypeText, model.ContentItemTypeAudio})
	}
	var items []*model.ContentItem
	if err := query.Order("created_at ASC, id ASC").Limit(limit).Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func validAutoSummaryContentTypeScope(scope model.AutoSummaryContentTypeScope) bool {
	switch scope {
	case model.AutoSummaryContentTypeScopeText, model.AutoSummaryContentTypeScopeAudio, model.AutoSummaryContentTypeScopeAll:
		return true
	default:
		return false
	}
}

func (r *contentItemRepository) AssignTags(ctx context.Context, itemID uint, tagIDs []uint) error {
	ids, err := uniquePositiveIDs(tagIDs)
	if err != nil || itemID == 0 {
		return v1.ErrBadRequest
	}
	if len(ids) == 0 {
		return nil
	}
	return r.DB(ctx).Transaction(func(tx *gorm.DB) error {
		if err := ensureContentItemExists(tx, itemID); err != nil {
			return err
		}
		if err := ensureTagsExist(tx, ids); err != nil {
			return err
		}
		return insertContentItemTags(tx, itemID, ids)
	})
}

func (r *contentItemRepository) RemoveTags(ctx context.Context, itemID uint, tagIDs []uint) error {
	ids, err := uniquePositiveIDs(tagIDs)
	if err != nil || itemID == 0 {
		return v1.ErrBadRequest
	}
	if len(ids) == 0 {
		return nil
	}
	return r.DB(ctx).Transaction(func(tx *gorm.DB) error {
		if err := ensureContentItemExists(tx, itemID); err != nil {
			return err
		}
		if err := ensureTagsExist(tx, ids); err != nil {
			return err
		}
		return tx.Table("content_item_tags").
			Where("content_item_id = ? AND tag_id IN ?", itemID, ids).
			Delete(nil).Error
	})
}

func uniquePositiveIDs(ids []uint) ([]uint, error) {
	seen := make(map[uint]struct{}, len(ids))
	unique := make([]uint, 0, len(ids))
	for _, id := range ids {
		if id == 0 {
			return nil, v1.ErrBadRequest
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		unique = append(unique, id)
	}
	return unique, nil
}

func ensureContentItemExists(tx *gorm.DB, itemID uint) error {
	var item model.ContentItem
	if err := tx.Select("id").Where("id = ?", itemID).Take(&item).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return v1.ErrContentItemNotFound
		}
		return err
	}
	return nil
}

func ensureTagsExist(tx *gorm.DB, tagIDs []uint) error {
	var count int64
	if err := tx.Model(&model.Tag{}).Where("id IN ?", tagIDs).Count(&count).Error; err != nil {
		return err
	}
	if count != int64(len(tagIDs)) {
		return v1.ErrTagNotFound
	}
	return nil
}

func insertContentItemTags(tx *gorm.DB, itemID uint, tagIDs []uint) error {
	var sql strings.Builder
	sql.WriteString("INSERT OR IGNORE INTO content_item_tags (content_item_id, tag_id) VALUES ")
	args := make([]interface{}, 0, len(tagIDs)*2)
	for i, tagID := range tagIDs {
		if i > 0 {
			sql.WriteString(", ")
		}
		sql.WriteString("(?, ?)")
		args = append(args, itemID, tagID)
	}
	return tx.Exec(sql.String(), args...).Error
}

func applyContentItemListFilter(query *gorm.DB, filter ContentItemListFilter) (*gorm.DB, error) {
	if keyword := strings.TrimSpace(filter.Keyword); keyword != "" {
		if contentItemSearchKeywordTooLong(keyword) {
			return nil, v1.ErrInvalidContentFilter
		}
		query = query.Joins("JOIN content_item_search_index ON content_item_search_index.rowid = content_items.id")
		if contentItemSearchUsesFTS(keyword) {
			query = query.Where("content_item_search_index MATCH ?", contentItemSearchMatchExpression(keyword))
		} else {
			query = applyContentItemSearchLike(query, keyword)
		}
	}
	if filter.ContentType != nil {
		if !validContentItemType(*filter.ContentType) {
			return nil, v1.ErrInvalidContentFilter
		}
		query = query.Where("content_items.type = ?", *filter.ContentType)
	}
	if filter.ProcessingStatus != nil {
		if !validContentItemProcessingStatus(*filter.ProcessingStatus) {
			return nil, v1.ErrInvalidContentFilter
		}
		query = query.Where("content_items.processing_status = ?", *filter.ProcessingStatus)
	}
	if filter.Mark != nil {
		column, ok := contentItemMarkColumn(*filter.Mark)
		if !ok {
			return nil, v1.ErrInvalidContentFilter
		}
		query = query.Where("content_items."+column+" = ?", true)
	}
	if filter.FeedID != nil {
		if *filter.FeedID == 0 {
			return nil, v1.ErrInvalidContentFilter
		}
		query = query.Where("content_items.feed_id = ?", *filter.FeedID)
	}
	if filter.TagID != nil {
		if *filter.TagID == 0 {
			return nil, v1.ErrInvalidContentFilter
		}
		query = query.Joins("JOIN content_item_tags ON content_item_tags.content_item_id = content_items.id AND content_item_tags.tag_id = ?", *filter.TagID)
	}
	if filter.FolderID != nil {
		if *filter.FolderID == 0 {
			return nil, v1.ErrInvalidContentFilter
		}
		query = query.Joins("JOIN feeds AS folder_filter_feeds ON folder_filter_feeds.id = content_items.feed_id AND folder_filter_feeds.folder_id = ?", *filter.FolderID)
	}
	return query, nil
}

func contentItemSearchKeywordTooLong(keyword string) bool {
	return utf8.RuneCountInString(strings.TrimSpace(keyword)) > contentItemSearchMaxKeywordRunes
}

func contentItemSearchUsesFTS(keyword string) bool {
	terms := contentItemSearchTerms(keyword)
	if len(terms) == 0 {
		return false
	}
	for _, term := range terms {
		if utf8.RuneCountInString(term) < 3 {
			return false
		}
	}
	return true
}

func contentItemSearchTerms(keyword string) []string {
	return strings.Fields(strings.TrimSpace(keyword))
}

func contentItemSearchMatchExpression(keyword string) string {
	terms := contentItemSearchTerms(keyword)
	quoted := make([]string, 0, len(terms))
	for _, term := range terms {
		quoted = append(quoted, `"`+strings.ReplaceAll(term, `"`, `""`)+`"`)
	}
	return strings.Join(quoted, " ")
}

func applyContentItemSearchLike(query *gorm.DB, keyword string) *gorm.DB {
	for _, term := range contentItemSearchTerms(keyword) {
		pattern := "%" + escapeContentItemSearchLike(term) + "%"
		query = query.Where(
			"(content_item_search_index.title LIKE ? ESCAPE '\\' OR content_item_search_index.feed_title LIKE ? ESCAPE '\\' OR content_item_search_index.available_text LIKE ? ESCAPE '\\' OR content_item_search_index.ai_summary_markdown LIKE ? ESCAPE '\\')",
			pattern,
			pattern,
			pattern,
			pattern,
		)
	}
	return query
}

func escapeContentItemSearchLike(keyword string) string {
	escaped := strings.ReplaceAll(keyword, `\`, `\\`)
	escaped = strings.ReplaceAll(escaped, `%`, `\%`)
	return strings.ReplaceAll(escaped, `_`, `\_`)
}

func validContentItemType(contentType model.ContentItemType) bool {
	switch contentType {
	case model.ContentItemTypeText, model.ContentItemTypeAudio:
		return true
	default:
		return false
	}
}

func validContentItemProcessingStatus(status model.ContentItemProcessingStatus) bool {
	switch status {
	case model.ContentItemProcessingStatusUnprocessed, model.ContentItemProcessingStatusCompleted:
		return true
	default:
		return false
	}
}

func (r *contentItemRepository) UpdateProcessingStatus(ctx context.Context, itemID uint, status model.ContentItemProcessingStatus) error {
	if itemID == 0 || !validContentItemProcessingStatus(status) {
		return v1.ErrBadRequest
	}
	result := r.DB(ctx).Model(&model.ContentItem{}).
		Where("id = ?", itemID).
		Update("processing_status", status)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return v1.ErrNotFound
	}
	return nil
}

func (r *contentItemRepository) UpdateMark(ctx context.Context, itemID uint, mark model.ContentItemMark, marked bool) error {
	if itemID == 0 {
		return v1.ErrBadRequest
	}
	column, ok := contentItemMarkColumn(mark)
	if !ok {
		return v1.ErrBadRequest
	}
	result := r.DB(ctx).Model(&model.ContentItem{}).
		Where("id = ?", itemID).
		Update(column, marked)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return v1.ErrNotFound
	}
	return nil
}

func contentItemMarkColumn(mark model.ContentItemMark) (string, bool) {
	switch mark {
	case model.ContentItemMarkLater:
		return "marked_later", true
	case model.ContentItemMarkFavorite:
		return "favorited", true
	default:
		return "", false
	}
}

func (r *contentItemRepository) UpdateAudioProgress(ctx context.Context, itemID uint, progressSeconds int) error {
	if itemID == 0 || progressSeconds < 0 {
		return v1.ErrBadRequest
	}
	result := r.DB(ctx).Model(&model.ContentItem{}).
		Where("id = ?", itemID).
		Update("audio_progress_seconds", progressSeconds)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return v1.ErrNotFound
	}
	return nil
}

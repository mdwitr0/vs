package repo

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/video-analitics/backend/pkg/status"
)

const scanTasksCollection = "scan_tasks"

// StageResult holds the result of a single stage (sitemap or page)
type StageResult struct {
	Status     status.Task `bson:"status" json:"status"`
	Total      int         `bson:"total" json:"total"`
	Success    int         `bson:"success" json:"success"`
	Failed     int         `bson:"failed" json:"failed"`
	Error      string      `bson:"error,omitempty" json:"error,omitempty"`
	StartedAt  *time.Time  `bson:"started_at,omitempty" json:"started_at,omitempty"`
	FinishedAt *time.Time  `bson:"finished_at,omitempty" json:"finished_at,omitempty"`
}

type ScanTask struct {
	ID            primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	SiteID        string             `bson:"site_id" json:"site_id"`
	Domain        string             `bson:"domain" json:"domain"`
	Status        status.Task        `bson:"status" json:"status"`
	Stage         status.Stage       `bson:"stage" json:"stage"`
	SitemapResult *StageResult       `bson:"sitemap_result,omitempty" json:"sitemap_result,omitempty"`
	PageResult    *StageResult       `bson:"page_result,omitempty" json:"page_result,omitempty"`
	CreatedAt     time.Time          `bson:"created_at" json:"created_at"`
	FinishedAt    *time.Time         `bson:"finished_at,omitempty" json:"finished_at,omitempty"`
	RetryCount    int                `bson:"retry_count" json:"retry_count"`
	NextRetryAt   *time.Time         `bson:"next_retry_at,omitempty" json:"next_retry_at,omitempty"`
	Version       int                `bson:"version" json:"-"`
}

type ScanTaskRepo struct {
	coll *mongo.Collection
}

func NewScanTaskRepo(db *mongo.Database) *ScanTaskRepo {
	coll := db.Collection(scanTasksCollection)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	indexes := []mongo.IndexModel{
		{Keys: bson.D{{Key: "created_at", Value: -1}}},
		{Keys: bson.D{{Key: "site_id", Value: 1}, {Key: "created_at", Value: -1}}},
		{Keys: bson.D{{Key: "status", Value: 1}, {Key: "created_at", Value: -1}}},
		{Keys: bson.D{{Key: "site_id", Value: 1}, {Key: "status", Value: 1}}},
		{Keys: bson.D{{Key: "stage", Value: 1}, {Key: "created_at", Value: -1}}},
		{Keys: bson.D{{Key: "status", Value: 1}, {Key: "next_retry_at", Value: 1}}},
	}
	coll.Indexes().CreateMany(ctx, indexes)

	return &ScanTaskRepo{coll: coll}
}

func (r *ScanTaskRepo) Create(ctx context.Context, task *ScanTask) error {
	now := time.Now()
	task.CreatedAt = now
	task.Status = status.TaskProcessing
	task.Stage = status.StageSitemap
	task.SitemapResult = &StageResult{
		Status:    status.TaskProcessing,
		StartedAt: &now,
	}
	task.Version = 0

	result, err := r.coll.InsertOne(ctx, task)
	if err != nil {
		return err
	}
	task.ID = result.InsertedID.(primitive.ObjectID)
	return nil
}

// CreateForPageStage создаёт задачу, которая сразу начинается с этапа page (пропуская sitemap)
func (r *ScanTaskRepo) CreateForPageStage(ctx context.Context, task *ScanTask, pendingURLs int) error {
	now := time.Now()
	task.CreatedAt = now
	task.Status = status.TaskProcessing
	task.Stage = status.StagePage
	task.PageResult = &StageResult{
		Status:    status.TaskProcessing,
		StartedAt: &now,
		Total:     pendingURLs,
	}
	task.Version = 0

	result, err := r.coll.InsertOne(ctx, task)
	if err != nil {
		return err
	}
	task.ID = result.InsertedID.(primitive.ObjectID)
	return nil
}

func (r *ScanTaskRepo) FindByID(ctx context.Context, id string) (*ScanTask, error) {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, err
	}

	var task ScanTask
	err = r.coll.FindOne(ctx, bson.M{"_id": oid}).Decode(&task)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	return &task, err
}

func (r *ScanTaskRepo) FindBySiteID(ctx context.Context, siteID string, limit int64) ([]ScanTask, error) {
	opts := options.Find().
		SetLimit(limit).
		SetSort(bson.D{{Key: "created_at", Value: -1}})

	cursor, err := r.coll.Find(ctx, bson.M{"site_id": siteID}, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var tasks []ScanTask
	if err := cursor.All(ctx, &tasks); err != nil {
		return nil, err
	}
	return tasks, nil
}

func (r *ScanTaskRepo) FindRecent(ctx context.Context, limit int64) ([]ScanTask, error) {
	opts := options.Find().
		SetLimit(limit).
		SetSort(bson.D{{Key: "created_at", Value: -1}})

	cursor, err := r.coll.Find(ctx, bson.M{}, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var tasks []ScanTask
	if err := cursor.All(ctx, &tasks); err != nil {
		return nil, err
	}
	return tasks, nil
}

func (r *ScanTaskRepo) FindWithPagination(ctx context.Context, siteID, domain, taskStatus string, limit, offset int64) ([]ScanTask, int64, error) {
	filter := bson.M{}
	if siteID != "" {
		filter["site_id"] = siteID
	}
	if domain != "" {
		filter["domain"] = bson.M{"$regex": domain, "$options": "i"}
	}
	if taskStatus != "" {
		filter["status"] = status.Task(taskStatus)
	}

	total, err := r.coll.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, err
	}

	opts := options.Find().
		SetLimit(limit).
		SetSkip(offset).
		SetSort(bson.D{{Key: "created_at", Value: -1}})

	cursor, err := r.coll.Find(ctx, filter, opts)
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(ctx)

	var tasks []ScanTask
	if err := cursor.All(ctx, &tasks); err != nil {
		return nil, 0, err
	}
	return tasks, total, nil
}

// FindByUserAccess returns tasks filtered by user access to sites using aggregation
// This is efficient even with millions of sites - filtering happens in MongoDB
func (r *ScanTaskRepo) FindByUserAccess(ctx context.Context, userID string, db *mongo.Database, siteID, domain, taskStatus string, limit, offset int64) ([]ScanTask, int64, error) {
	userOID, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return nil, 0, err
	}

	// Build match stage for tasks
	taskMatch := bson.M{}
	if siteID != "" {
		taskMatch["site_id"] = siteID
	}
	if domain != "" {
		taskMatch["domain"] = bson.M{"$regex": domain, "$options": "i"}
	}
	if taskStatus != "" {
		taskMatch["status"] = status.Task(taskStatus)
	}

	// Pipeline: join with sites, filter by owner_id or user_sites
	pipeline := mongo.Pipeline{
		// Convert site_id string to ObjectId for lookup
		{{Key: "$addFields", Value: bson.M{
			"site_oid": bson.M{"$toObjectId": "$site_id"},
		}}},
		// Join with sites collection
		{{Key: "$lookup", Value: bson.M{
			"from":         "sites",
			"localField":   "site_oid",
			"foreignField": "_id",
			"as":           "site",
		}}},
		{{Key: "$unwind", Value: bson.M{"path": "$site", "preserveNullAndEmptyArrays": false}}},
		// Join with user_sites to check shared access
		{{Key: "$lookup", Value: bson.M{
			"from": "user_sites",
			"let":  bson.M{"site_id": "$site_oid"},
			"pipeline": mongo.Pipeline{
				{{Key: "$match", Value: bson.M{
					"$expr": bson.M{
						"$and": bson.A{
							bson.M{"$eq": bson.A{"$site_id", "$$site_id"}},
							bson.M{"$eq": bson.A{"$user_id", userOID}},
						},
					},
				}}},
			},
			"as": "user_site_access",
		}}},
		// Filter: user is owner OR has shared access
		{{Key: "$match", Value: bson.M{
			"$or": bson.A{
				bson.M{"site.owner_id": userOID},
				bson.M{"user_site_access": bson.M{"$ne": bson.A{}}},
			},
		}}},
		// Remove helper fields
		{{Key: "$project", Value: bson.M{
			"site_oid":         0,
			"site":             0,
			"user_site_access": 0,
		}}},
	}

	// Add task filters if any
	if len(taskMatch) > 0 {
		pipeline = append(mongo.Pipeline{{{Key: "$match", Value: taskMatch}}}, pipeline...)
	}

	// Count total (without pagination)
	countPipeline := append(pipeline, bson.D{{Key: "$count", Value: "total"}})
	countCursor, err := r.coll.Aggregate(ctx, countPipeline)
	if err != nil {
		return nil, 0, err
	}
	var countResult []struct {
		Total int64 `bson:"total"`
	}
	if err := countCursor.All(ctx, &countResult); err != nil {
		return nil, 0, err
	}
	countCursor.Close(ctx)

	var total int64
	if len(countResult) > 0 {
		total = countResult[0].Total
	}

	// Add sort, skip, limit for data query
	pipeline = append(pipeline,
		bson.D{{Key: "$sort", Value: bson.M{"created_at": -1}}},
		bson.D{{Key: "$skip", Value: offset}},
		bson.D{{Key: "$limit", Value: limit}},
	)

	cursor, err := r.coll.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(ctx)

	var tasks []ScanTask
	if err := cursor.All(ctx, &tasks); err != nil {
		return nil, 0, err
	}

	return tasks, total, nil
}

func (r *ScanTaskRepo) HasActiveTask(ctx context.Context, siteID string) (bool, error) {
	count, err := r.coll.CountDocuments(ctx, bson.M{
		"site_id": siteID,
		"status":  bson.M{"$in": status.ActiveTaskStatuses()},
	})
	return count > 0, err
}

func (r *ScanTaskRepo) FindLatest(ctx context.Context, siteID string) (*ScanTask, error) {
	opts := options.FindOne().SetSort(bson.D{{Key: "created_at", Value: -1}})
	var task ScanTask
	err := r.coll.FindOne(ctx, bson.M{"site_id": siteID}, opts).Decode(&task)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	return &task, err
}

// UpdateSitemapResult updates the sitemap stage result
func (r *ScanTaskRepo) UpdateSitemapResult(ctx context.Context, taskID string, result *StageResult) error {
	oid, err := primitive.ObjectIDFromHex(taskID)
	if err != nil {
		return err
	}

	_, err = r.coll.UpdateOne(
		ctx,
		bson.M{"_id": oid},
		bson.M{
			"$set": bson.M{"sitemap_result": result},
			"$inc": bson.M{"version": 1},
		},
	)
	return err
}

// UpdatePageResult updates the page stage result
func (r *ScanTaskRepo) UpdatePageResult(ctx context.Context, taskID string, result *StageResult) error {
	oid, err := primitive.ObjectIDFromHex(taskID)
	if err != nil {
		return err
	}

	_, err = r.coll.UpdateOne(
		ctx,
		bson.M{"_id": oid},
		bson.M{
			"$set": bson.M{"page_result": result},
			"$inc": bson.M{"version": 1},
		},
	)
	return err
}

// CompleteSitemapStage marks sitemap stage as completed and starts page stage
// sitemapTotal is the real count from sitemap_urls collection
func (r *ScanTaskRepo) CompleteSitemapStage(ctx context.Context, taskID string, sitemapTotal int64) error {
	oid, err := primitive.ObjectIDFromHex(taskID)
	if err != nil {
		return err
	}

	now := time.Now()

	_, err = r.coll.UpdateOne(ctx, bson.M{"_id": oid}, bson.M{
		"$set": bson.M{
			"stage":                      status.StagePage,
			"sitemap_result.status":      status.TaskCompleted,
			"sitemap_result.finished_at": now,
			"sitemap_result.total":       sitemapTotal,
			"sitemap_result.success":     sitemapTotal,
			"page_result.status":         status.TaskProcessing,
			"page_result.started_at":     now,
			"page_result.total":          sitemapTotal,
		},
		"$inc": bson.M{"version": 1},
	})
	return err
}

// CompleteSitemapStageOnly marks sitemap stage as completed WITHOUT starting page stage
// Used when AutoContinue=false - user will manually trigger page crawl
// sitemapTotal is the real count from sitemap_urls collection
func (r *ScanTaskRepo) CompleteSitemapStageOnly(ctx context.Context, taskID string, sitemapTotal int64) error {
	oid, err := primitive.ObjectIDFromHex(taskID)
	if err != nil {
		return err
	}

	now := time.Now()

	_, err = r.coll.UpdateOne(ctx, bson.M{"_id": oid}, bson.M{
		"$set": bson.M{
			"status":                     status.TaskCompleted,
			"stage":                      status.StageSitemap,
			"sitemap_result.status":      status.TaskCompleted,
			"sitemap_result.finished_at": now,
			"sitemap_result.total":       sitemapTotal,
			"sitemap_result.success":     sitemapTotal,
			"finished_at":                now,
		},
		"$inc": bson.M{"version": 1},
	})
	return err
}

// FailSitemapStage marks sitemap stage and entire task as failed
func (r *ScanTaskRepo) FailSitemapStage(ctx context.Context, taskID string, sitemapResult *StageResult) error {
	oid, err := primitive.ObjectIDFromHex(taskID)
	if err != nil {
		return err
	}

	now := time.Now()
	sitemapResult.Status = status.TaskFailed
	sitemapResult.FinishedAt = &now

	_, err = r.coll.UpdateOne(
		ctx,
		bson.M{"_id": oid},
		bson.M{
			"$set": bson.M{
				"status":         status.TaskFailed,
				"sitemap_result": sitemapResult,
				"finished_at":    now,
			},
			"$inc": bson.M{"version": 1},
		},
	)
	return err
}

// CompletePageStage marks page stage and entire task as completed
// Note: does NOT overwrite success/failed counters - they are updated by IncrementPageProgress
func (r *ScanTaskRepo) CompletePageStage(ctx context.Context, taskID string, pageResult *StageResult) error {
	oid, err := primitive.ObjectIDFromHex(taskID)
	if err != nil {
		return err
	}

	now := time.Now()

	update := bson.M{
		"status":                  status.TaskCompleted,
		"stage":                   status.StageDone,
		"page_result.status":      status.TaskCompleted,
		"page_result.finished_at": now,
		"finished_at":             now,
	}

	// only set error if provided
	if pageResult != nil && pageResult.Error != "" {
		update["page_result.error"] = pageResult.Error
	}

	_, err = r.coll.UpdateOne(
		ctx,
		bson.M{"_id": oid},
		bson.M{
			"$set": update,
			"$inc": bson.M{"version": 1},
		},
	)
	return err
}

// FailPageStage marks page stage and entire task as failed
// Note: does NOT overwrite success/failed counters - they are updated by IncrementPageProgress
func (r *ScanTaskRepo) FailPageStage(ctx context.Context, taskID string, pageResult *StageResult) error {
	oid, err := primitive.ObjectIDFromHex(taskID)
	if err != nil {
		return err
	}

	now := time.Now()

	update := bson.M{
		"status":                  status.TaskFailed,
		"stage":                   status.StageDone,
		"page_result.status":      status.TaskFailed,
		"page_result.finished_at": now,
		"finished_at":             now,
	}

	// set error if provided
	if pageResult != nil && pageResult.Error != "" {
		update["page_result.error"] = pageResult.Error
	}

	_, err = r.coll.UpdateOne(
		ctx,
		bson.M{"_id": oid},
		bson.M{
			"$set": update,
			"$inc": bson.M{"version": 1},
		},
	)
	return err
}

// UpdatePageProgress updates page result progress (total, success, failed)
func (r *ScanTaskRepo) UpdatePageProgress(ctx context.Context, taskID string, total, success, failed int) error {
	oid, err := primitive.ObjectIDFromHex(taskID)
	if err != nil {
		return err
	}

	_, err = r.coll.UpdateOne(
		ctx,
		bson.M{"_id": oid},
		bson.M{
			"$set": bson.M{
				"page_result.total":   total,
				"page_result.success": success,
				"page_result.failed":  failed,
			},
		},
	)
	return err
}

// IncrementPageProgress atomically increments success or failed counter for page stage
func (r *ScanTaskRepo) IncrementPageProgress(ctx context.Context, taskID string, success bool) error {
	oid, err := primitive.ObjectIDFromHex(taskID)
	if err != nil {
		return err
	}

	field := "page_result.failed"
	if success {
		field = "page_result.success"
	}

	_, err = r.coll.UpdateOne(
		ctx,
		bson.M{"_id": oid},
		bson.M{"$inc": bson.M{field: 1}},
	)
	return err
}

func (r *ScanTaskRepo) MarkCancelled(ctx context.Context, taskID string) error {
	oid, err := primitive.ObjectIDFromHex(taskID)
	if err != nil {
		return err
	}

	now := time.Now()
	result, err := r.coll.UpdateOne(
		ctx,
		bson.M{
			"_id":    oid,
			"status": bson.M{"$in": status.ActiveTaskStatuses()},
		},
		bson.M{
			"$set": bson.M{
				"status":      status.TaskCancelled,
				"finished_at": now,
			},
			"$inc": bson.M{"version": 1},
		},
	)
	if err != nil {
		return err
	}

	if result.MatchedCount == 0 {
		return status.ErrConcurrentUpdate
	}
	return nil
}

func (r *ScanTaskRepo) CancelMany(ctx context.Context, taskIDs []string) (int64, error) {
	var oids []primitive.ObjectID
	for _, id := range taskIDs {
		oid, err := primitive.ObjectIDFromHex(id)
		if err != nil {
			continue
		}
		oids = append(oids, oid)
	}

	if len(oids) == 0 {
		return 0, nil
	}

	now := time.Now()
	result, err := r.coll.UpdateMany(
		ctx,
		bson.M{
			"_id":    bson.M{"$in": oids},
			"status": bson.M{"$in": status.ActiveTaskStatuses()},
		},
		bson.M{
			"$set": bson.M{
				"status":      status.TaskCancelled,
				"finished_at": now,
			},
			"$inc": bson.M{"version": 1},
		},
	)
	if err != nil {
		return 0, err
	}
	return result.ModifiedCount, nil
}

// CancelBySiteID cancels all active tasks for a site (used when site becomes unavailable)
func (r *ScanTaskRepo) CancelBySiteID(ctx context.Context, siteID string) (int64, error) {
	now := time.Now()
	result, err := r.coll.UpdateMany(
		ctx,
		bson.M{
			"site_id": siteID,
			"status":  bson.M{"$in": status.ActiveTaskStatuses()},
		},
		bson.M{
			"$set": bson.M{
				"status":      status.TaskCancelled,
				"finished_at": now,
			},
			"$inc": bson.M{"version": 1},
		},
	)
	if err != nil {
		return 0, err
	}
	return result.ModifiedCount, nil
}

func (r *ScanTaskRepo) DeleteBySiteID(ctx context.Context, siteID string) (int64, error) {
	result, err := r.coll.DeleteMany(ctx, bson.M{"site_id": siteID})
	if err != nil {
		return 0, err
	}
	return result.DeletedCount, nil
}

// ActiveTaskInfo содержит этап и прогресс активной задачи
type ActiveTaskInfo struct {
	Stage   status.Stage
	Total   int
	Success int
	Failed  int
}

// GetActiveTasksInfo возвращает информацию об активных задачах для списка сайтов
func (r *ScanTaskRepo) GetActiveTasksInfo(ctx context.Context, siteIDs []string) (map[string]*ActiveTaskInfo, error) {
	if len(siteIDs) == 0 {
		return make(map[string]*ActiveTaskInfo), nil
	}

	cursor, err := r.coll.Find(ctx, bson.M{
		"site_id": bson.M{"$in": siteIDs},
		"status":  bson.M{"$in": status.ActiveTaskStatuses()},
	})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var tasks []ScanTask
	if err := cursor.All(ctx, &tasks); err != nil {
		return nil, err
	}

	result := make(map[string]*ActiveTaskInfo)
	for _, task := range tasks {
		if _, exists := result[task.SiteID]; !exists {
			info := &ActiveTaskInfo{
				Stage: task.Stage,
			}
			// Берём прогресс в зависимости от этапа
			if task.Stage == status.StageSitemap && task.SitemapResult != nil {
				info.Total = task.SitemapResult.Total
				info.Success = task.SitemapResult.Success
				info.Failed = task.SitemapResult.Failed
			} else if task.PageResult != nil {
				info.Total = task.PageResult.Total
				info.Success = task.PageResult.Success
				info.Failed = task.PageResult.Failed
			}
			result[task.SiteID] = info
		}
	}

	return result, nil
}

// GetActiveStages возвращает только этапы активных задач (для обратной совместимости)
func (r *ScanTaskRepo) GetActiveStages(ctx context.Context, siteIDs []string) (map[string]status.Stage, error) {
	info, err := r.GetActiveTasksInfo(ctx, siteIDs)
	if err != nil {
		return nil, err
	}

	result := make(map[string]status.Stage)
	for siteID, taskInfo := range info {
		result[siteID] = taskInfo.Stage
	}
	return result, nil
}

// LastScanInfo содержит информацию о последнем завершённом сканировании
type LastScanInfo struct {
	Success int
	Total   int
	Status  status.Task
}

// GetLastCompletedTasksInfo возвращает информацию о последней завершённой задаче для каждого сайта
func (r *ScanTaskRepo) GetLastCompletedTasksInfo(ctx context.Context, siteIDs []string) (map[string]*LastScanInfo, error) {
	if len(siteIDs) == 0 {
		return make(map[string]*LastScanInfo), nil
	}

	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: bson.M{
			"site_id": bson.M{"$in": siteIDs},
			"status":  bson.M{"$in": []status.Task{status.TaskCompleted, status.TaskFailed}},
		}}},
		{{Key: "$sort", Value: bson.M{"created_at": -1}}},
		{{Key: "$group", Value: bson.M{
			"_id":       "$site_id",
			"last_task": bson.M{"$first": "$$ROOT"},
		}}},
	}

	cursor, err := r.coll.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var results []struct {
		SiteID   string   `bson:"_id"`
		LastTask ScanTask `bson:"last_task"`
	}
	if err := cursor.All(ctx, &results); err != nil {
		return nil, err
	}

	info := make(map[string]*LastScanInfo)
	for _, r := range results {
		task := r.LastTask
		scanInfo := &LastScanInfo{
			Status: task.Status,
		}
		if task.PageResult != nil {
			scanInfo.Success = task.PageResult.Success
			scanInfo.Total = task.PageResult.Total
		}
		info[r.SiteID] = scanInfo
	}

	return info, nil
}

// FindStaleTasks finds tasks that are stuck in pending or processing state
func (r *ScanTaskRepo) FindStaleTasks(ctx context.Context, pendingTimeout, processingTimeout time.Duration) ([]ScanTask, error) {
	now := time.Now()
	pendingCutoff := now.Add(-pendingTimeout)
	processingCutoff := now.Add(-processingTimeout)

	cursor, err := r.coll.Find(ctx, bson.M{
		"$or": []bson.M{
			{
				"status":     status.TaskPending,
				"created_at": bson.M{"$lt": pendingCutoff},
			},
			{
				"status":     status.TaskProcessing,
				"created_at": bson.M{"$lt": processingCutoff},
			},
		},
	})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var tasks []ScanTask
	if err := cursor.All(ctx, &tasks); err != nil {
		return nil, err
	}
	return tasks, nil
}

// MarkFailed marks a task as failed with an error message
// Also updates sitemap_result and page_result statuses if they are in processing state
func (r *ScanTaskRepo) MarkFailed(ctx context.Context, taskID, errMsg string) error {
	return r.MarkFailedWithRetry(ctx, taskID, errMsg, nil)
}

// MarkFailedWithRetry marks a task as failed and schedules next retry
func (r *ScanTaskRepo) MarkFailedWithRetry(ctx context.Context, taskID, errMsg string, nextRetryAt *time.Time) error {
	oid, err := primitive.ObjectIDFromHex(taskID)
	if err != nil {
		return err
	}

	now := time.Now()

	// First, get current task to check stage
	var task ScanTask
	if err := r.coll.FindOne(ctx, bson.M{"_id": oid}).Decode(&task); err != nil {
		return err
	}

	update := bson.M{
		"status":      status.TaskFailed,
		"finished_at": now,
	}

	if nextRetryAt != nil {
		update["next_retry_at"] = *nextRetryAt
	}

	// Update sitemap_result if it's still processing
	if task.SitemapResult != nil && (task.SitemapResult.Status == status.TaskPending || task.SitemapResult.Status == status.TaskProcessing) {
		update["sitemap_result.status"] = status.TaskFailed
		update["sitemap_result.finished_at"] = now
		if errMsg != "" {
			update["sitemap_result.error"] = errMsg
		}
	}

	// Update page_result if it's still processing
	if task.PageResult != nil && (task.PageResult.Status == status.TaskPending || task.PageResult.Status == status.TaskProcessing) {
		update["page_result.status"] = status.TaskFailed
		update["page_result.finished_at"] = now
		if errMsg != "" {
			update["page_result.error"] = errMsg
		}
	}

	_, err = r.coll.UpdateOne(
		ctx,
		bson.M{"_id": oid},
		bson.M{
			"$set": update,
			"$inc": bson.M{"version": 1},
		},
	)
	return err
}

// MarkProcessingByHex marks a task as processing by its hex ID
func (r *ScanTaskRepo) MarkProcessingByHex(ctx context.Context, taskID string) error {
	oid, err := primitive.ObjectIDFromHex(taskID)
	if err != nil {
		return err
	}

	_, err = r.coll.UpdateOne(
		ctx,
		bson.M{
			"_id":    oid,
			"status": status.TaskPending,
		},
		bson.M{
			"$set": bson.M{"status": status.TaskProcessing},
			"$inc": bson.M{"version": 1},
		},
	)
	return err
}

// UpdateProgress updates task progress (legacy - for old CrawlProgress messages)
func (r *ScanTaskRepo) UpdateProgress(ctx context.Context, progress interface{}) error {
	// Legacy method - the new two-stage system uses UpdatePageProgress
	return nil
}

// UpdateFromResult updates task from crawl result (legacy - for old CrawlResult messages)
func (r *ScanTaskRepo) UpdateFromResult(ctx context.Context, result interface{}) error {
	// Legacy method - the new two-stage system uses CompleteSitemapStage/CompletePageStage
	return nil
}

// FindFailedTasksForRetry finds failed tasks eligible for retry
// Returns tasks where: status=failed, retry_count < maxRetries, next_retry_at <= now
func (r *ScanTaskRepo) FindFailedTasksForRetry(ctx context.Context, maxRetries int) ([]ScanTask, error) {
	now := time.Now()

	cursor, err := r.coll.Find(ctx, bson.M{
		"status":      status.TaskFailed,
		"retry_count": bson.M{"$lt": maxRetries},
		"$or": []bson.M{
			{"next_retry_at": bson.M{"$lte": now}},
			{"next_retry_at": bson.M{"$exists": false}},
			{"next_retry_at": nil},
		},
	})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var tasks []ScanTask
	if err := cursor.All(ctx, &tasks); err != nil {
		return nil, err
	}
	return tasks, nil
}

// IncrementRetryAndReset increments retry count and resets task to pending status
func (r *ScanTaskRepo) IncrementRetryAndReset(ctx context.Context, taskID string) error {
	oid, err := primitive.ObjectIDFromHex(taskID)
	if err != nil {
		return err
	}

	now := time.Now()

	// Get current task to restore stage results
	var task ScanTask
	if err := r.coll.FindOne(ctx, bson.M{"_id": oid}).Decode(&task); err != nil {
		return err
	}

	update := bson.M{
		"status":        status.TaskProcessing,
		"finished_at":   nil,
		"next_retry_at": nil,
	}

	// Reset the failed stage result to processing
	if task.Stage == status.StageSitemap && task.SitemapResult != nil {
		update["sitemap_result.status"] = status.TaskProcessing
		update["sitemap_result.started_at"] = now
		update["sitemap_result.finished_at"] = nil
		update["sitemap_result.error"] = ""
	} else if task.Stage == status.StagePage && task.PageResult != nil {
		update["page_result.status"] = status.TaskProcessing
		update["page_result.started_at"] = now
		update["page_result.finished_at"] = nil
		update["page_result.error"] = ""
	}

	_, err = r.coll.UpdateOne(
		ctx,
		bson.M{"_id": oid, "status": status.TaskFailed},
		bson.M{
			"$set": update,
			"$inc": bson.M{"version": 1, "retry_count": 1},
		},
	)
	return err
}

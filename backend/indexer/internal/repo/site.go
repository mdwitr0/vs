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

const sitesCollection = "sites"

type SitemapInfo struct {
	URL           string     `bson:"url" json:"url"`
	URLsCount     int        `bson:"urls_count" json:"urls_count"`
	LastParsedAt  *time.Time `bson:"last_parsed_at,omitempty" json:"last_parsed_at,omitempty"`
	LastURLsFound int        `bson:"last_urls_found" json:"last_urls_found"`
	ErrorCount    int        `bson:"error_count" json:"error_count"`
	LastError     string     `bson:"last_error,omitempty" json:"last_error,omitempty"`
}

type Cookie struct {
	Name     string `bson:"name" json:"name"`
	Value    string `bson:"value" json:"value"`
	Domain   string `bson:"domain" json:"domain"`
	Path     string `bson:"path" json:"path"`
	Expires  int64  `bson:"expires,omitempty" json:"expires,omitempty"`
	HTTPOnly bool   `bson:"http_only" json:"http_only"`
	Secure   bool   `bson:"secure" json:"secure"`
}

type Site struct {
	ID               primitive.ObjectID   `bson:"_id,omitempty" json:"id"`
	OwnerID          primitive.ObjectID   `bson:"owner_id,omitempty" json:"owner_id,omitempty"`
	Domain           string               `bson:"domain" json:"domain"`
	Status           status.Site          `bson:"status" json:"status"`
	CMS              string               `bson:"cms,omitempty" json:"cms,omitempty"`
	HasSitemap       bool                 `bson:"has_sitemap" json:"has_sitemap"`
	SitemapStatus    status.SitemapStatus `bson:"sitemap_status" json:"sitemap_status"`
	CrawlStrategy    status.CrawlStrategy `bson:"crawl_strategy" json:"crawl_strategy"`
	SitemapURLs      []string             `bson:"sitemap_urls,omitempty" json:"sitemap_urls,omitempty"`
	Sitemaps         []SitemapInfo        `bson:"sitemaps,omitempty" json:"sitemaps,omitempty"`
	TotalURLsCount   int                  `bson:"total_urls_count" json:"total_urls_count"`
	LastScanAt       *time.Time           `bson:"last_scan_at,omitempty" json:"last_scan_at,omitempty"`
	NextScanAt       *time.Time           `bson:"next_scan_at,omitempty" json:"next_scan_at,omitempty"`
	FailureCount     int                  `bson:"failure_count" json:"failure_count"`
	ScanIntervalH    int                  `bson:"scan_interval_h" json:"scan_interval_h"`
	ScannerType      status.ScannerType   `bson:"scanner_type" json:"scanner_type"`
	CaptchaType      string               `bson:"captcha_type,omitempty" json:"captcha_type,omitempty"`
	Cookies          []Cookie             `bson:"cookies,omitempty" json:"-"`
	CookiesUpdatedAt *time.Time           `bson:"cookies_updated_at,omitempty" json:"cookies_updated_at,omitempty"`
	FreezeReason     string               `bson:"freeze_reason,omitempty" json:"freeze_reason,omitempty"`
	MovedToDomain    string               `bson:"moved_to_domain,omitempty" json:"moved_to_domain,omitempty"`
	MovedAt          *time.Time           `bson:"moved_at,omitempty" json:"moved_at,omitempty"`
	OriginalDomain   string               `bson:"original_domain,omitempty" json:"original_domain,omitempty"`
	CreatedAt        time.Time            `bson:"created_at" json:"created_at"`
	Version          int                  `bson:"version" json:"-"`
}

type SiteRepo struct {
	coll *mongo.Collection
}

func NewSiteRepo(db *mongo.Database) *SiteRepo {
	coll := db.Collection(sitesCollection)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	indexes := []mongo.IndexModel{
		{Keys: bson.D{{Key: "domain", Value: 1}}, Options: options.Index().SetUnique(true)},
		{Keys: bson.D{{Key: "status", Value: 1}, {Key: "created_at", Value: -1}}},
		{Keys: bson.D{{Key: "status", Value: 1}, {Key: "next_scan_at", Value: 1}}},
		{Keys: bson.D{{Key: "owner_id", Value: 1}}},
	}
	coll.Indexes().CreateMany(ctx, indexes)

	return &SiteRepo{coll: coll}
}

func (r *SiteRepo) Create(ctx context.Context, site *Site) error {
	now := time.Now()
	site.CreatedAt = now
	site.Status = status.SitePending
	site.Version = 0
	if site.ScanIntervalH == 0 {
		site.ScanIntervalH = 24
	}
	if site.ScannerType == "" {
		site.ScannerType = status.ScannerHTTP
	}
	result, err := r.coll.InsertOne(ctx, site)
	if err != nil {
		return err
	}
	site.ID = result.InsertedID.(primitive.ObjectID)
	return nil
}

func (r *SiteRepo) FindByID(ctx context.Context, id string) (*Site, error) {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, err
	}

	var site Site
	err = r.coll.FindOne(ctx, bson.M{"_id": oid}).Decode(&site)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	return &site, err
}

func (r *SiteRepo) FindByDomain(ctx context.Context, domain string) (*Site, error) {
	var site Site
	err := r.coll.FindOne(ctx, bson.M{"domain": domain}).Decode(&site)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	return &site, err
}

type SiteFilter struct {
	Status       string
	ScannedSince *time.Time
	SiteIDs      []string
	ExcludeIDs   []string
	Limit        int64
	Offset       int64
}

func (r *SiteRepo) FindAll(ctx context.Context, filter SiteFilter) ([]Site, int64, error) {
	query := bson.M{}
	if filter.Status != "" {
		query["status"] = filter.Status
	}
	if filter.ScannedSince != nil {
		query["last_scan_at"] = bson.M{"$gte": *filter.ScannedSince}
	}
	if len(filter.SiteIDs) > 0 {
		var oids []primitive.ObjectID
		for _, id := range filter.SiteIDs {
			if oid, err := primitive.ObjectIDFromHex(id); err == nil {
				oids = append(oids, oid)
			}
		}
		query["_id"] = bson.M{"$in": oids}
	}
	if len(filter.ExcludeIDs) > 0 {
		var oids []primitive.ObjectID
		for _, id := range filter.ExcludeIDs {
			if oid, err := primitive.ObjectIDFromHex(id); err == nil {
				oids = append(oids, oid)
			}
		}
		query["_id"] = bson.M{"$nin": oids}
	}

	total, err := r.coll.CountDocuments(ctx, query)
	if err != nil {
		return nil, 0, err
	}

	opts := options.Find().
		SetLimit(filter.Limit).
		SetSkip(filter.Offset).
		SetSort(bson.D{
			{Key: "status", Value: 1},
			{Key: "created_at", Value: -1},
		})

	cursor, err := r.coll.Find(ctx, query, opts)
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(ctx)

	var sites []Site
	if err := cursor.All(ctx, &sites); err != nil {
		return nil, 0, err
	}

	return sites, total, nil
}

func (r *SiteRepo) FindByIDs(ctx context.Context, ids []string) ([]Site, error) {
	var oids []primitive.ObjectID
	for _, id := range ids {
		oid, err := primitive.ObjectIDFromHex(id)
		if err != nil {
			continue
		}
		oids = append(oids, oid)
	}

	cursor, err := r.coll.Find(ctx, bson.M{"_id": bson.M{"$in": oids}})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var sites []Site
	if err := cursor.All(ctx, &sites); err != nil {
		return nil, err
	}
	return sites, nil
}

func (r *SiteRepo) Update(ctx context.Context, site *Site) error {
	_, err := r.coll.UpdateOne(
		ctx,
		bson.M{"_id": site.ID},
		bson.M{
			"$set": site,
			"$inc": bson.M{"version": 1},
		},
	)
	return err
}

// SafeUpdateStatus safely updates site status with optimistic locking
func (r *SiteRepo) SafeUpdateStatus(ctx context.Context, siteID string, expectedStatus, newStatus status.Site, updates bson.M) error {
	if !status.CanSiteTransition(expectedStatus, newStatus) {
		return status.ErrInvalidTransition
	}

	oid, err := primitive.ObjectIDFromHex(siteID)
	if err != nil {
		return err
	}

	if updates == nil {
		updates = bson.M{}
	}
	updates["status"] = newStatus

	result, err := r.coll.UpdateOne(
		ctx,
		bson.M{
			"_id":    oid,
			"status": expectedStatus,
		},
		bson.M{
			"$set": updates,
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

// SafeUpdateStatusFromAny updates status without checking current state
func (r *SiteRepo) SafeUpdateStatusFromAny(ctx context.Context, siteID string, newStatus status.Site, updates bson.M) error {
	oid, err := primitive.ObjectIDFromHex(siteID)
	if err != nil {
		return err
	}

	if updates == nil {
		updates = bson.M{}
	}
	updates["status"] = newStatus

	_, err = r.coll.UpdateOne(
		ctx,
		bson.M{"_id": oid},
		bson.M{
			"$set": updates,
			"$inc": bson.M{"version": 1},
		},
	)
	return err
}

func (r *SiteRepo) MarkSuccess(ctx context.Context, siteID string, scanIntervalH int) error {
	now := time.Now()
	if scanIntervalH == 0 {
		scanIntervalH = 24
	}
	nextScan := now.Add(time.Duration(scanIntervalH) * time.Hour)

	return r.SafeUpdateStatusFromAny(ctx, siteID, status.SiteActive, bson.M{
		"last_scan_at":  now,
		"next_scan_at":  nextScan,
		"failure_count": 0,
	})
}

func (r *SiteRepo) MarkFailure(ctx context.Context, siteID string, isDomainExpired bool) error {
	oid, err := primitive.ObjectIDFromHex(siteID)
	if err != nil {
		return err
	}

	now := time.Now()

	if isDomainExpired {
		return r.SafeUpdateStatusFromAny(ctx, siteID, status.SiteDead, bson.M{
			"last_scan_at": now,
		})
	}

	nextScan := now.Add(12 * time.Hour)

	// 3 consecutive failures -> dead, otherwise stay active but increment counter
	_, err = r.coll.UpdateOne(
		ctx,
		bson.M{"_id": oid},
		bson.A{
			bson.M{
				"$set": bson.M{
					"last_scan_at":  now,
					"failure_count": bson.M{"$add": bson.A{"$failure_count", 1}},
					"status": bson.M{
						"$cond": bson.A{
							bson.M{"$gte": bson.A{bson.M{"$add": bson.A{"$failure_count", 1}}, 3}},
							status.SiteDead,
							"$status", // keep current status until 3 failures
						},
					},
					"next_scan_at": bson.M{
						"$cond": bson.A{
							bson.M{"$gte": bson.A{bson.M{"$add": bson.A{"$failure_count", 1}}, 3}},
							"$next_scan_at",
							nextScan,
						},
					},
				},
			},
		},
	)
	return err
}

func (r *SiteRepo) FindDueForScan(ctx context.Context, limit int64) ([]Site, error) {
	now := time.Now()

	opts := options.Find().
		SetLimit(limit).
		SetSort(bson.D{{Key: "next_scan_at", Value: 1}})

	cursor, err := r.coll.Find(ctx, bson.M{
		"status":       bson.M{"$in": status.ScannableSiteStatuses()},
		"next_scan_at": bson.M{"$lte": now},
	}, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var sites []Site
	if err := cursor.All(ctx, &sites); err != nil {
		return nil, err
	}
	return sites, nil
}

func (r *SiteRepo) MarkQueued(ctx context.Context, siteIDs []string) error {
	var oids []primitive.ObjectID
	for _, id := range siteIDs {
		oid, err := primitive.ObjectIDFromHex(id)
		if err != nil {
			continue
		}
		oids = append(oids, oid)
	}

	if len(oids) == 0 {
		return nil
	}

	nextScan := time.Now().Add(time.Hour)

	_, err := r.coll.UpdateMany(
		ctx,
		bson.M{"_id": bson.M{"$in": oids}},
		bson.M{
			"$set": bson.M{"next_scan_at": nextScan},
			"$inc": bson.M{"version": 1},
		},
	)
	return err
}

func (r *SiteRepo) MarkFrozen(ctx context.Context, siteID string, reason string) error {
	now := time.Now()
	return r.SafeUpdateStatusFromAny(ctx, siteID, status.SiteFrozen, bson.M{
		"last_scan_at":  now,
		"freeze_reason": reason,
		"failure_count": 0,
	})
}

func (r *SiteRepo) MarkAsMoved(ctx context.Context, siteID, movedToDomain string) error {
	now := time.Now()
	return r.SafeUpdateStatusFromAny(ctx, siteID, status.SiteMoved, bson.M{
		"moved_to_domain": movedToDomain,
		"moved_at":        now,
	})
}

func (r *SiteRepo) Unfreeze(ctx context.Context, siteID string, scannerType status.ScannerType) error {
	now := time.Now()
	updates := bson.M{
		"freeze_reason": "",
		"next_scan_at":  now,
		"failure_count": 0,
	}
	if scannerType != "" {
		updates["scanner_type"] = scannerType
	}

	return r.SafeUpdateStatus(ctx, siteID, status.SiteFrozen, status.SiteActive, updates)
}

type DetectionUpdate struct {
	CMS           string
	HasSitemap    bool
	SitemapStatus status.SitemapStatus
	CrawlStrategy status.CrawlStrategy
	SitemapURLs   []string
	ScannerType   status.ScannerType
	CaptchaType   string
	Cookies       []Cookie
}

func (r *SiteRepo) UpdateFromDetection(ctx context.Context, siteID string, update DetectionUpdate) error {
	now := time.Now()
	setUpdate := bson.M{
		"cms":            update.CMS,
		"has_sitemap":    update.HasSitemap,
		"sitemap_status": update.SitemapStatus,
		"crawl_strategy": update.CrawlStrategy,
		"next_scan_at":   now,
	}
	if len(update.SitemapURLs) > 0 {
		setUpdate["sitemap_urls"] = update.SitemapURLs
	}
	if update.ScannerType != "" {
		setUpdate["scanner_type"] = update.ScannerType
	}
	if update.CaptchaType != "" {
		setUpdate["captcha_type"] = update.CaptchaType
	}
	if len(update.Cookies) > 0 {
		setUpdate["cookies"] = update.Cookies
	}

	return r.SafeUpdateStatus(ctx, siteID, status.SitePending, status.SiteActive, setUpdate)
}

type SitemapStats struct {
	URL       string
	URLsFound int
	Error     error
}

func (r *SiteRepo) UpdateSitemapStats(ctx context.Context, siteID string, stats []SitemapStats) error {
	oid, err := primitive.ObjectIDFromHex(siteID)
	if err != nil {
		return err
	}

	now := time.Now()
	var sitemaps []SitemapInfo
	var totalURLs int

	for _, s := range stats {
		info := SitemapInfo{
			URL:           s.URL,
			LastParsedAt:  &now,
			LastURLsFound: s.URLsFound,
		}

		if s.Error != nil {
			info.ErrorCount = 1
			info.LastError = s.Error.Error()
		} else {
			info.URLsCount = s.URLsFound
			totalURLs += s.URLsFound
		}

		sitemaps = append(sitemaps, info)
	}

	_, err = r.coll.UpdateOne(
		ctx,
		bson.M{"_id": oid},
		bson.M{
			"$set": bson.M{
				"sitemaps":         sitemaps,
				"total_urls_count": totalURLs,
			},
			"$inc": bson.M{"version": 1},
		},
	)
	return err
}

func (r *SiteRepo) IncrementSitemapError(ctx context.Context, siteID string, sitemapURL string, errMsg string) error {
	oid, err := primitive.ObjectIDFromHex(siteID)
	if err != nil {
		return err
	}

	now := time.Now()
	_, err = r.coll.UpdateOne(
		ctx,
		bson.M{
			"_id":          oid,
			"sitemaps.url": sitemapURL,
		},
		bson.M{
			"$inc": bson.M{"sitemaps.$.error_count": 1},
			"$set": bson.M{
				"sitemaps.$.last_error":     errMsg,
				"sitemaps.$.last_parsed_at": now,
			},
		},
	)
	return err
}

func (r *SiteRepo) UpdateCookies(ctx context.Context, siteID string, cookies []Cookie) error {
	oid, err := primitive.ObjectIDFromHex(siteID)
	if err != nil {
		return err
	}

	now := time.Now()
	_, err = r.coll.UpdateOne(
		ctx,
		bson.M{"_id": oid},
		bson.M{
			"$set": bson.M{
				"cookies":            cookies,
				"cookies_updated_at": now,
			},
			"$inc": bson.M{"version": 1},
		},
	)
	return err
}

func (r *SiteRepo) GetCookies(ctx context.Context, siteID string) ([]Cookie, error) {
	site, err := r.FindByID(ctx, siteID)
	if err != nil {
		return nil, err
	}
	if site == nil {
		return nil, nil
	}
	return site.Cookies, nil
}

func (r *SiteRepo) Delete(ctx context.Context, id string) error {
	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return err
	}
	_, err = r.coll.DeleteOne(ctx, bson.M{"_id": objectID})
	return err
}

func (r *SiteRepo) ResetToPending(ctx context.Context, siteID string) error {
	return r.SafeUpdateStatusFromAny(ctx, siteID, status.SitePending, bson.M{
		"failure_count": 0,
		"freeze_reason": "",
	})
}

func (r *SiteRepo) IncrementFailureCount(ctx context.Context, siteID string) error {
	oid, err := primitive.ObjectIDFromHex(siteID)
	if err != nil {
		return err
	}

	_, err = r.coll.UpdateOne(
		ctx,
		bson.M{"_id": oid},
		bson.M{
			"$inc": bson.M{"failure_count": 1, "version": 1},
		},
	)
	return err
}

func (r *SiteRepo) FindPendingSites(ctx context.Context, olderThan time.Duration, limit int64) ([]Site, error) {
	threshold := time.Now().Add(-olderThan)

	opts := options.Find().
		SetLimit(limit).
		SetSort(bson.D{{Key: "created_at", Value: 1}})

	cursor, err := r.coll.Find(ctx, bson.M{
		"status":     status.SitePending,
		"created_at": bson.M{"$lte": threshold},
	}, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var sites []Site
	if err := cursor.All(ctx, &sites); err != nil {
		return nil, err
	}
	return sites, nil
}

// FindByUserAccess returns sites the user has access to using aggregation
// Efficient even with millions of user_sites records - filtering happens in MongoDB
func (r *SiteRepo) FindByUserAccess(ctx context.Context, userID string, isAdmin bool, filter SiteFilter) ([]Site, int64, error) {
	if isAdmin {
		return r.FindAll(ctx, filter)
	}

	userOID, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return nil, 0, err
	}

	// Build initial match stage
	initialMatch := bson.M{}
	if filter.Status != "" {
		initialMatch["status"] = filter.Status
	}
	if filter.ScannedSince != nil {
		initialMatch["last_scan_at"] = bson.M{"$gte": *filter.ScannedSince}
	}
	if len(filter.SiteIDs) > 0 {
		var oids []primitive.ObjectID
		for _, id := range filter.SiteIDs {
			if oid, err := primitive.ObjectIDFromHex(id); err == nil {
				oids = append(oids, oid)
			}
		}
		initialMatch["_id"] = bson.M{"$in": oids}
	}
	if len(filter.ExcludeIDs) > 0 {
		var oids []primitive.ObjectID
		for _, id := range filter.ExcludeIDs {
			if oid, err := primitive.ObjectIDFromHex(id); err == nil {
				oids = append(oids, oid)
			}
		}
		initialMatch["_id"] = bson.M{"$nin": oids}
	}

	// Pipeline: join with user_sites to check shared access
	pipeline := mongo.Pipeline{}

	// Add initial filters if any
	if len(initialMatch) > 0 {
		pipeline = append(pipeline, bson.D{{Key: "$match", Value: initialMatch}})
	}

	// Lookup user_sites for this user
	pipeline = append(pipeline,
		bson.D{{Key: "$lookup", Value: bson.M{
			"from": "user_sites",
			"let":  bson.M{"site_id": "$_id"},
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
		bson.D{{Key: "$match", Value: bson.M{
			"$or": bson.A{
				bson.M{"owner_id": userOID},
				bson.M{"user_site_access": bson.M{"$ne": bson.A{}}},
			},
		}}},
		// Remove helper field
		bson.D{{Key: "$project", Value: bson.M{"user_site_access": 0}}},
	)

	// Count total
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

	// Add sort, skip, limit
	pipeline = append(pipeline,
		bson.D{{Key: "$sort", Value: bson.D{
			{Key: "status", Value: 1},
			{Key: "created_at", Value: -1},
		}}},
		bson.D{{Key: "$skip", Value: filter.Offset}},
		bson.D{{Key: "$limit", Value: filter.Limit}},
	)

	cursor, err := r.coll.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(ctx)

	var sites []Site
	if err := cursor.All(ctx, &sites); err != nil {
		return nil, 0, err
	}

	return sites, total, nil
}

func (r *SiteRepo) HasUserAccess(ctx context.Context, siteID, userID string, isAdmin bool, userSiteRepo *UserSiteRepo) (bool, error) {
	if isAdmin {
		return true, nil
	}

	site, err := r.FindByID(ctx, siteID)
	if err != nil {
		return false, err
	}
	if site == nil {
		return false, nil
	}

	userOID, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return false, err
	}

	if site.OwnerID == userOID {
		return true, nil
	}

	exists, err := userSiteRepo.ExistsByUserAndSite(ctx, userID, siteID)
	if err != nil {
		return false, err
	}
	return exists, nil
}

// GetAccessibleSiteIDs returns list of site IDs the user has access to
func (r *SiteRepo) GetAccessibleSiteIDs(ctx context.Context, userID string, userSiteRepo *UserSiteRepo) ([]string, error) {
	userOID, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return nil, err
	}

	// Get sites owned by the user
	cursor, err := r.coll.Find(ctx, bson.M{"owner_id": userOID}, options.Find().SetProjection(bson.M{"_id": 1}))
	if err != nil {
		return nil, err
	}

	var siteIDs []string
	for cursor.Next(ctx) {
		var result struct {
			ID primitive.ObjectID `bson:"_id"`
		}
		if err := cursor.Decode(&result); err != nil {
			continue
		}
		siteIDs = append(siteIDs, result.ID.Hex())
	}
	cursor.Close(ctx)

	// Get sites shared with the user via user_sites
	sharedSiteIDs, err := userSiteRepo.GetSiteIDsByUserID(ctx, userID)
	if err != nil {
		return siteIDs, nil // Return owned sites even if shared query fails
	}

	// Merge and deduplicate
	siteIDSet := make(map[string]bool)
	for _, id := range siteIDs {
		siteIDSet[id] = true
	}
	for _, id := range sharedSiteIDs {
		siteIDSet[id] = true
	}

	result := make([]string, 0, len(siteIDSet))
	for id := range siteIDSet {
		result = append(result, id)
	}

	return result, nil
}

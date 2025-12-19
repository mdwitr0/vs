package violations

import (
	"context"

	"github.com/video-analitics/backend/pkg/meili"
	"go.mongodb.org/mongo-driver/mongo"
)

type ContentCountUpdater interface {
	UpdateViolationsCount(ctx context.Context, id string, violationsCount, sitesCount int64) error
}

type Service struct {
	repo           *Repository
	calculator     *Calculator
	contentUpdater ContentCountUpdater
}

func NewService(db *mongo.Database, meiliClient *meili.Client) *Service {
	repo := NewRepository(db)
	matcher := NewMatcher(meiliClient)
	calculator := NewCalculator(repo, matcher)

	return &Service{
		repo:       repo,
		calculator: calculator,
	}
}

func (s *Service) SetContentUpdater(updater ContentCountUpdater) {
	s.contentUpdater = updater
}

func (s *Service) RefreshForContent(ctx context.Context, content ContentInfo) (*ContentStats, error) {
	stats, err := s.calculator.CalculateForContent(ctx, content)
	if err != nil {
		return nil, err
	}

	if s.contentUpdater != nil && stats != nil {
		s.contentUpdater.UpdateViolationsCount(ctx, content.ID, stats.ViolationsCount, stats.SitesCount)
	}

	return stats, nil
}

func (s *Service) RefreshAll(ctx context.Context, contents []ContentInfo) (int64, error) {
	updated, err := s.calculator.CalculateForAllContent(ctx, contents)
	if err != nil {
		return updated, err
	}

	if s.contentUpdater != nil {
		for _, content := range contents {
			stats, _ := s.repo.GetContentStats(ctx, content.ID)
			if stats != nil {
				s.contentUpdater.UpdateViolationsCount(ctx, content.ID, stats.ViolationsCount, stats.SitesCount)
			}
		}
	}

	return updated, nil
}

// RefreshForSite обновляет violations только для страниц конкретного сайта
func (s *Service) RefreshForSite(ctx context.Context, siteID string, contents []ContentInfo) (int64, error) {
	updated, err := s.calculator.CalculateForSite(ctx, siteID, contents)
	if err != nil {
		return updated, err
	}

	if s.contentUpdater != nil {
		for _, content := range contents {
			stats, _ := s.repo.GetContentStats(ctx, content.ID)
			if stats != nil {
				s.contentUpdater.UpdateViolationsCount(ctx, content.ID, stats.ViolationsCount, stats.SitesCount)
			}
		}
	}

	return updated, nil
}

func (s *Service) GetByContentID(ctx context.Context, contentID string, limit, offset int64) ([]Violation, int64, error) {
	return s.repo.FindByContentID(ctx, contentID, limit, offset)
}

func (s *Service) GetBySiteID(ctx context.Context, siteID string, limit, offset int64) ([]Violation, int64, error) {
	return s.repo.FindBySiteID(ctx, siteID, limit, offset)
}

func (s *Service) GetAllByContentID(ctx context.Context, contentID string) ([]Violation, error) {
	return s.repo.FindAllByContentID(ctx, contentID)
}

func (s *Service) GetContentStats(ctx context.Context, contentID string) (*ContentStats, error) {
	return s.repo.GetContentStats(ctx, contentID)
}

func (s *Service) GetSiteStats(ctx context.Context, siteID string) (*SiteStats, error) {
	return s.repo.GetSiteStats(ctx, siteID)
}

func (s *Service) GetAllContentStats(ctx context.Context) (map[string]*ContentStats, error) {
	return s.repo.GetAllContentStats(ctx)
}

func (s *Service) GetAllSiteStats(ctx context.Context) (map[string]*SiteStats, error) {
	return s.repo.GetAllSiteStats(ctx)
}

func (s *Service) DeleteByPageID(ctx context.Context, pageID string) error {
	return s.repo.DeleteByPageID(ctx, pageID)
}

func (s *Service) DeleteByContentID(ctx context.Context, contentID string) error {
	return s.repo.DeleteByContentID(ctx, contentID)
}

func (s *Service) CountBySiteID(ctx context.Context, siteID string) (int64, error) {
	return s.repo.CountBySiteID(ctx, siteID)
}

func (s *Service) CountByContentID(ctx context.Context, contentID string) (int64, error) {
	return s.repo.CountByContentID(ctx, contentID)
}

func (s *Service) GetPageIDsBySiteID(ctx context.Context, siteID string) ([]string, error) {
	return s.repo.GetPageIDsBySiteID(ctx, siteID)
}

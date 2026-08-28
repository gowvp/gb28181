package recording

import (
	"context"
	"log/slog"
	"time"

	"github.com/ixugo/goddd/pkg/orm"
	"github.com/ixugo/goddd/pkg/reason"
	"github.com/ixugo/goddd/pkg/web"
	"github.com/jinzhu/copier"
	"gorm.io/gorm"
)

// RecordingStorer Instantiation interface
type RecordingStorer interface {
	List(context.Context, orm.Pager, ...orm.QueryOption) ([]*Recording, int64, error)
	Get(context.Context, *Recording, ...orm.QueryOption) error
	Create(context.Context, *Recording) error
	Update(context.Context, *Recording, func(*Recording), ...orm.QueryOption) error
	Delete(context.Context, *Recording, ...orm.QueryOption) error
	Count(context.Context, ...orm.QueryOption) (int64, error)

	Session(context.Context, ...func(*gorm.DB) error) error
	UpdateWithSession(*gorm.DB, *Recording, func(b *Recording) error, ...orm.QueryOption) error
}

// ListRecordings 分页查询录像列表，支持通道ID和时间范围筛选
func (c Core) ListRecordings(ctx context.Context, in *FindRecordingInput) ([]*Recording, int64, error) {
	query := orm.NewQuery(4).OrderBy("started_at DESC")

	if in.CID != "" {
		query.Where("cid = ?", in.CID)
	}
	if in.App != "" {
		query.Where("app = ?", in.App)
	}
	if in.Stream != "" {
		query.Where("stream = ?", in.Stream)
	}
	if in.StartMs > 0 && in.EndMs > 0 {
		query.Where("started_at >= ? AND ended_at <= ?", in.StartAt(), in.EndAt())
	}

	items, total, err := c.store.Recording().List(ctx, in, query.Encode()...)
	if err != nil {
		return nil, 0, reason.ErrDB.Withf(`Find in[%+v] err[%s]`, in, err.Error())
	}
	for _, item := range items {
		if ctx, ok := ctx.(web.Context); ok {
			item.Path = ctx.BaseURLJoin("/static/recordings", item.Path)
		}
	}
	return items, total, nil
}

// GetRecording Query a single object
func (c Core) GetRecording(ctx context.Context, id int64) (*Recording, error) {
	out := Recording{ID: id}
	if err := c.store.Recording().Get(ctx, &out, orm.Where("id=?", id)); err != nil {
		if orm.IsErrRecordNotFound(err) {
			return nil, reason.ErrNotFound.Withf(`Get id[%v] err[%s]`, id, err.Error())
		}
		return nil, reason.ErrDB.Withf(`Get id[%v] err[%s]`, id, err.Error())
	}
	return &out, nil
}

// CreateRecording Insert into database
func (c Core) CreateRecording(ctx context.Context, in *AddRecordingInput) (*Recording, error) {
	var out Recording
	if err := copier.Copy(&out, in); err != nil {
		slog.ErrorContext(ctx, "Copy", "err", err)
	}

	if err := c.store.Recording().Create(ctx, &out); err != nil {
		return nil, reason.ErrDB.Withf(`Create err[%s]`, err.Error())
	}
	return &out, nil
}

// UpdateRecording Update object information
func (c Core) UpdateRecording(ctx context.Context, in *EditRecordingInput, id int64) (*Recording, error) {
	var out Recording
	if err := c.store.Recording().Update(ctx, &out, func(b *Recording) {
		if err := copier.Copy(b, in); err != nil {
			slog.ErrorContext(ctx, "Copy", "err", err)
		}
	}, orm.Where("id=?", id)); err != nil {
		return nil, reason.ErrDB.Withf(`Edit id[%v] err[%s]`, id, err.Error())
	}
	return &out, nil
}

// DeleteRecording Delete object
func (c Core) DeleteRecording(ctx context.Context, id int64) (*Recording, error) {
	var out Recording
	if err := c.store.Recording().Delete(ctx, &out, orm.Where("id=?", id)); err != nil {
		return nil, reason.ErrDB.Withf(`Del id[%v] err[%s]`, id, err.Error())
	}
	return &out, nil
}

// GetTimeline 获取时间轴数据，返回指定时间范围内的录像时段列表
func (c Core) GetTimeline(ctx context.Context, in *TimelineInput) ([]TimeRange, error) {
	if in.CID == "" {
		return nil, reason.ErrBadRequest.Withf("cid is required")
	}
	if in.StartMs <= 0 || in.EndMs <= 0 {
		return nil, reason.ErrBadRequest.Withf("start_ms and end_ms are required")
	}

	query := orm.NewQuery(2).OrderBy("started_at ASC")
	query.Where("cid = ?", in.CID)
	// 查询时间范围内有重叠的录像
	query.Where("started_at < ? AND ended_at > ?", in.EndAt(), in.StartAt())

	var recordings []*Recording
	// 使用默认分页器避免 nil pointer
	pager := &defaultPager{limit: 1000}
	recordings, _, err := c.store.Recording().List(ctx, pager, query.Encode()...)
	if err != nil {
		return nil, reason.ErrDB.Withf(`GetTimeline err[%s]`, err.Error())
	}

	result := make([]TimeRange, 0, len(recordings))
	for _, r := range recordings {
		result = append(result, TimeRange{
			ID:          r.ID,
			StartMs:     r.StartedAt.UnixMilli(),
			EndMs:       r.EndedAt.UnixMilli(),
			Duration:    r.Duration,
			ObjectCount: r.ObjectCount,
			DeleteFlag:  r.DeleteFlag,
		})
	}
	return result, nil
}

// defaultPager 内部使用的分页器，避免传入 nil 导致空指针
type defaultPager struct {
	limit int
}

func (p *defaultPager) Offset() int { return 0 }
func (p *defaultPager) Limit() int  { return p.limit }

// cidCount 用于接收 GROUP BY 查询结果
type cidCount struct {
	CID   string `gorm:"column:cid"`
	Count int64  `gorm:"column:cnt"`
}

// HasRecordings 批量检查通道是否有录像
// 使用 WHERE IN + GROUP BY 一次性查询所有通道是否有录像
func (c Core) HasRecordings(ctx context.Context, cids []string) (map[string]bool, error) {
	result := make(map[string]bool, len(cids))
	if len(cids) == 0 {
		return result, nil
	}

	// 使用 Session 执行自定义 SQL：SELECT cid, COUNT(*) as cnt FROM recordings WHERE cid IN (?) GROUP BY cid
	var counts []cidCount
	err := c.store.Recording().Session(ctx, func(db *gorm.DB) error {
		return db.Model(&Recording{}).
			Select("cid, COUNT(*) as cnt").
			Where("cid IN ?", cids).
			Group("cid").
			Find(&counts).Error
	})
	if err != nil {
		return result, err
	}

	// 转换结果
	for _, c := range counts {
		result[c.CID] = c.Count > 0
	}
	return result, nil
}

// GetMonthlyStats 获取月度录像统计
// 返回指定月份每天是否有录像的位图字符串
func (c Core) GetMonthlyStats(ctx context.Context, in *MonthlyStatsInput) (*MonthlyStatsOutput, error) {
	if in.Year <= 0 || in.Month < 1 || in.Month > 12 {
		return nil, reason.ErrBadRequest.Withf("invalid year or month")
	}

	// 计算该月的第一天和最后一天
	firstDay := time.Date(in.Year, time.Month(in.Month), 1, 0, 0, 0, 0, time.Local)
	lastDay := firstDay.AddDate(0, 1, 0).Add(-time.Nanosecond)
	daysInMonth := lastDay.Day()

	// 查询该月有录像的日期
	query := orm.NewQuery(2)
	query.Where("started_at >= ? AND started_at <= ?", orm.Time{Time: firstDay}, orm.Time{Time: lastDay})
	if in.CID != "" {
		query.Where("cid = ?", in.CID)
	}

	var recordings []*Recording
	// 使用默认分页器避免 nil pointer
	pager := &defaultPager{limit: 10000}
	recordings, _, err := c.store.Recording().List(ctx, pager, query.Encode()...)
	if err != nil {
		return nil, reason.ErrDB.Withf(`GetMonthlyStats err[%s]`, err.Error())
	}

	// 统计每天是否有录像
	dayHasVideo := make([]bool, daysInMonth)
	for _, r := range recordings {
		day := r.StartedAt.Day()
		if day >= 1 && day <= daysInMonth {
			dayHasVideo[day-1] = true
		}
	}

	// 构建位图字符串
	bitmap := make([]byte, daysInMonth)
	for i, has := range dayHasVideo {
		if has {
			bitmap[i] = '1'
		} else {
			bitmap[i] = '0'
		}
	}

	return &MonthlyStatsOutput{
		Year:     in.Year,
		Month:    in.Month,
		Days:     daysInMonth,
		HasVideo: string(bitmap),
	}, nil
}

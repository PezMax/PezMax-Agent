package domain

type FileSearchRequest struct {
	Query    string `json:"query"`
	Keyword  string `json:"keyword,omitempty"`
	School   string `json:"school,omitempty"`
	Subject  string `json:"subject,omitempty"`
	Year     int    `json:"year,omitempty"`
	Type     int    `json:"type,omitempty"`
	PageNum  int    `json:"pageNum,omitempty"`
	PageSize int    `json:"pageSize,omitempty"`
}

type FileItem struct {
	FileID   int64  `json:"fileId"`
	UserID   int64  `json:"userId,omitempty"`
	Name     string `json:"fileName"`
	URL      string `json:"fileUrl,omitempty"`
	Size     int64  `json:"fileSize,omitempty"`
	Format   string `json:"fileFormat,omitempty"`
	Year     int    `json:"fileYear,omitempty"`
	Type     int    `json:"fileType,omitempty"`
	School   string `json:"fileSchool,omitempty"`
	Subject  string `json:"fileSubject,omitempty"`
	Reviewer string `json:"reviewer,omitempty"`
	Status   int    `json:"fileStatus,omitempty"`
	Remark   string `json:"remark,omitempty"`
}

type DownloadItem struct {
	DownloadID int64  `json:"downloadId"`
	FileID     int64  `json:"fileId,omitempty"`
	UserID     int64  `json:"userId,omitempty"`
	CreateBy   string `json:"creatBy,omitempty"`
	CreateTime string `json:"creatTime,omitempty"`
	Remark     string `json:"remark,omitempty"`
}

type NotificationItem struct {
	NotifyID              int64  `json:"notifyId"`
	NotifyType            string `json:"notifyType,omitempty"`
	Title                 string `json:"title,omitempty"`
	Content               string `json:"content,omitempty"`
	Status                string `json:"status,omitempty"`
	Sort                  int64  `json:"sort,omitempty"`
	DisplayMode           string `json:"displayMode,omitempty"`
	UploadUserID          int64  `json:"uploadUserId,omitempty"`
	MaterialID            int64  `json:"materialId,omitempty"`
	MaterialTitleSnapshot string `json:"materialTitleSnapshot,omitempty"`
	PublishStart          string `json:"publishStart,omitempty"`
	PublishEnd            string `json:"publishEnd,omitempty"`
	CreateTime            string `json:"createTime,omitempty"`
	Remark                string `json:"remark,omitempty"`
}

type UploaderRankItem struct {
	UserID     int64  `json:"userId"`
	UserName   string `json:"userName,omitempty"`
	NickName   string `json:"nickName,omitempty"`
	Avatar     string `json:"avatar,omitempty"`
	Count      int64  `json:"count"`
	Remark     string `json:"remark,omitempty"`
	CreateTime string `json:"createTime,omitempty"`
}

type FileSearchResponse struct {
	Intent      string             `json:"intent"`
	Filters     FileSearchRequest  `json:"filters"`
	Items       []FileItem         `json:"items"`
	Results     []FileSearchResult `json:"results"`
	Suggestions []string           `json:"suggestions,omitempty"`
	Summary     string             `json:"summary"`
}

type FileSearchResult struct {
	File    FileItem `json:"file"`
	Score   int      `json:"score"`
	Reasons []string `json:"reasons"`
}

type FileRecommendRequest struct {
	FileID  int64  `json:"fileId,omitempty"`
	Keyword string `json:"keyword,omitempty"`
	School  string `json:"school,omitempty"`
	Subject string `json:"subject,omitempty"`
	Year    int    `json:"year,omitempty"`
	Type    int    `json:"type,omitempty"`
	Limit   int    `json:"limit,omitempty"`
}

type FileRecommendResponse struct {
	Intent          string             `json:"intent"`
	SeedFile        *FileItem          `json:"seedFile,omitempty"`
	Filters         FileSearchRequest  `json:"filters"`
	Recommendations []FileSearchResult `json:"recommendations"`
	Suggestions     []string           `json:"suggestions,omitempty"`
	Summary         string             `json:"summary"`
}

type StudyPlanRequest struct {
	UserID      int64   `json:"userId,omitempty"`
	Goal        string  `json:"goal"`
	Subject     string  `json:"subject,omitempty"`
	Days        int     `json:"days,omitempty"`
	HoursPerDay float64 `json:"hoursPerDay,omitempty"`
	School      string  `json:"school,omitempty"`
	Year        int     `json:"year,omitempty"`
}

type StudyPlanResponse struct {
	Intent           string             `json:"intent"`
	Goal             string             `json:"goal"`
	Subject          string             `json:"subject,omitempty"`
	Days             int                `json:"days"`
	HoursPerDay      float64            `json:"hoursPerDay"`
	HasPlatformFiles bool               `json:"hasPlatformFiles"`
	MaterialAnalysis string             `json:"materialAnalysis,omitempty"`
	Plan             []StudyPlanDay     `json:"plan"`
	RecommendedFiles []FileSearchResult `json:"recommendedFiles,omitempty"`
	WebSources       []WebSearchResult  `json:"webSources,omitempty"`
	Suggestions      []string           `json:"suggestions,omitempty"`
	Summary          string             `json:"summary"`
}

type StudyPlanDay struct {
	Day              int         `json:"day"`
	Title            string      `json:"title"`
	Focus            string      `json:"focus,omitempty"`
	Tasks            []StudyTask `json:"tasks"`
	RecommendedFiles []FileItem  `json:"recommendedFiles,omitempty"`
}

type StudyTask struct {
	Type    string `json:"type"`
	Title   string `json:"title"`
	Detail  string `json:"detail,omitempty"`
	Minutes int    `json:"minutes"`
}

type MockExamRequest struct {
	UserID        int64   `json:"userId,omitempty"`
	Subject       string  `json:"subject,omitempty"`
	School        string  `json:"school,omitempty"`
	Year          int     `json:"year,omitempty"`
	FileIDs       []int64 `json:"fileIds,omitempty"`
	QuestionCount int     `json:"questionCount,omitempty"`
	Difficulty    string  `json:"difficulty,omitempty"`
	Goal          string  `json:"goal,omitempty"`
}

type MockExamResponse struct {
	Intent           string             `json:"intent"`
	Subject          string             `json:"subject,omitempty"`
	School           string             `json:"school,omitempty"`
	Year             int                `json:"year,omitempty"`
	HasPastPapers    bool               `json:"hasPastPapers"`
	PaperAnalysis    string             `json:"paperAnalysis,omitempty"`
	SourceFiles      []FileItem         `json:"sourceFiles,omitempty"`
	WebSources       []WebSearchResult  `json:"webSources,omitempty"`
	Questions        []MockQuestion     `json:"questions"`
	RecommendedFiles []FileSearchResult `json:"recommendedFiles,omitempty"`
	Suggestions      []string           `json:"suggestions,omitempty"`
	Summary          string             `json:"summary"`
}

type MockQuestion struct {
	Number      int      `json:"number"`
	Type        string   `json:"type"`
	Topic       string   `json:"topic,omitempty"`
	Difficulty  string   `json:"difficulty,omitempty"`
	Stem        string   `json:"stem"`
	Options     []string `json:"options,omitempty"`
	Answer      string   `json:"answer,omitempty"`
	Analysis    string   `json:"analysis,omitempty"`
	SourceBasis string   `json:"sourceBasis,omitempty"`
}

type WebSearchResult struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Snippet string `json:"snippet"`
	Source  string `json:"source,omitempty"`
}

type FavoriteOrganizeRequest struct {
	UserID   int64  `json:"userId"`
	GroupBy  string `json:"groupBy,omitempty"`
	PageNum  int    `json:"pageNum,omitempty"`
	PageSize int    `json:"pageSize,omitempty"`
}

type FavoriteOrganizeResponse struct {
	Intent      string          `json:"intent"`
	UserID      int64           `json:"userId"`
	Total       int             `json:"total"`
	Groups      []FavoriteGroup `json:"groups"`
	Suggestions []string        `json:"suggestions,omitempty"`
	Summary     string          `json:"summary"`
}

type FavoriteGroup struct {
	Key      string     `json:"key"`
	Label    string     `json:"label"`
	Count    int        `json:"count"`
	Priority string     `json:"priority"`
	Items    []FileItem `json:"items"`
}

type ReportSummarizeRequest struct {
	ReportID int64  `json:"reportId,omitempty"`
	FileID   int64  `json:"fileId,omitempty"`
	UserID   int64  `json:"userId,omitempty"`
	Result   string `json:"result,omitempty"`
	PageNum  int    `json:"pageNum,omitempty"`
	PageSize int    `json:"pageSize,omitempty"`
}

type ReportSummarizeResponse struct {
	Intent      string          `json:"intent"`
	Filters     ReportQuery     `json:"filters"`
	Reports     []ReportSummary `json:"reports"`
	RiskLevel   string          `json:"riskLevel"`
	Suggestions []string        `json:"suggestions,omitempty"`
	Summary     string          `json:"summary"`
}

type ReportQuery struct {
	ReportID int64  `json:"reportId,omitempty"`
	FileID   int64  `json:"fileId,omitempty"`
	UserID   int64  `json:"userId,omitempty"`
	Result   string `json:"result,omitempty"`
	PageNum  int    `json:"pageNum,omitempty"`
	PageSize int    `json:"pageSize,omitempty"`
}

type ReportItem struct {
	ReportID   int64  `json:"reportId"`
	FileID     int64  `json:"fileId,omitempty"`
	UserID     int64  `json:"userId,omitempty"`
	Reason     string `json:"reason,omitempty"`
	Result     string `json:"result,omitempty"`
	CreateBy   string `json:"createBy,omitempty"`
	CreateTime string `json:"createTime,omitempty"`
	UpdateBy   string `json:"updateBy,omitempty"`
	UpdateTime string `json:"updateTime,omitempty"`
	Remark     string `json:"remark,omitempty"`
}

type ReportSummary struct {
	Report       ReportItem      `json:"report"`
	File         *FileItem       `json:"file,omitempty"`
	Audit        AuditSuggestion `json:"audit"`
	Clues        []string        `json:"clues"`
	NextActions  []string        `json:"nextActions"`
	TimelineText string          `json:"timelineText,omitempty"`
}

type OpsInsightRequest struct {
	PageNum              int    `json:"pageNum,omitempty"`
	PageSize             int    `json:"pageSize,omitempty"`
	School               string `json:"school,omitempty"`
	Subject              string `json:"subject,omitempty"`
	Year                 int    `json:"year,omitempty"`
	IncludeNotifications bool   `json:"includeNotifications,omitempty"`
}

type OpsInsightResponse struct {
	Intent            string                        `json:"intent"`
	Filters           FileSearchRequest             `json:"filters"`
	Overview          OpsOverview                   `json:"overview"`
	HotFiles          []HotFileInsight              `json:"hotFiles"`
	LowQualityFiles   []QualityIssueInsight         `json:"lowQualityFiles"`
	ReportPressure    []ReportPressureInsight       `json:"reportPressure"`
	RankTrends        []UploaderRankInsight         `json:"rankTrends"`
	NotificationReach []NotificationReachSuggestion `json:"notificationReach,omitempty"`
	Suggestions       []string                      `json:"suggestions,omitempty"`
	Summary           string                        `json:"summary"`
}

type OpsOverview struct {
	FileCount         int    `json:"fileCount"`
	DownloadCount     int    `json:"downloadCount"`
	ReportCount       int    `json:"reportCount"`
	NotificationCount int    `json:"notificationCount"`
	HighRiskCount     int    `json:"highRiskCount"`
	HotSubject        string `json:"hotSubject,omitempty"`
	HotSchool         string `json:"hotSchool,omitempty"`
}

type HotFileInsight struct {
	File          FileItem `json:"file"`
	DownloadCount int      `json:"downloadCount"`
	ReportCount   int      `json:"reportCount"`
	HotScore      int      `json:"hotScore"`
	Reasons       []string `json:"reasons"`
}

type QualityIssueInsight struct {
	File      FileItem `json:"file"`
	RiskLevel string   `json:"riskLevel"`
	Score     int      `json:"score"`
	Reasons   []string `json:"reasons"`
}

type ReportPressureInsight struct {
	FileID      int64     `json:"fileId"`
	File        *FileItem `json:"file,omitempty"`
	ReportCount int       `json:"reportCount"`
	RiskLevel   string    `json:"riskLevel"`
	Reasons     []string  `json:"reasons"`
}

type UploaderRankInsight struct {
	Rank    int              `json:"rank"`
	User    UploaderRankItem `json:"user"`
	Trend   string           `json:"trend"`
	Insight string           `json:"insight"`
}

type NotificationReachSuggestion struct {
	Type          string  `json:"type"`
	Title         string  `json:"title"`
	Audience      string  `json:"audience"`
	Priority      string  `json:"priority"`
	DraftContent  string  `json:"draftContent"`
	RelatedFileID int64   `json:"relatedFileId,omitempty"`
	Confidence    float64 `json:"confidence"`
}

type MetadataSuggestRequest struct {
	FileName     string `json:"fileName"`
	OriginalName string `json:"originalName,omitempty"`
	SchoolHint   string `json:"schoolHint,omitempty"`
	SubjectHint  string `json:"subjectHint,omitempty"`
	RemarkHint   string `json:"remarkHint,omitempty"`
}

type MetadataSuggestion struct {
	FileName   string   `json:"fileName"`
	School     string   `json:"fileSchool,omitempty"`
	Subject    string   `json:"fileSubject,omitempty"`
	Year       int      `json:"fileYear,omitempty"`
	Type       int      `json:"fileType,omitempty"`
	TypeName   string   `json:"fileTypeName,omitempty"`
	Remark     string   `json:"remark,omitempty"`
	Confidence float64  `json:"confidence"`
	Reasons    []string `json:"reasons"`
}

type AuditSuggestRequest struct {
	File FileItem `json:"file"`
}

type AuditSuggestion struct {
	SuggestedAction string   `json:"suggestedAction"`
	RiskLevel       string   `json:"riskLevel"`
	RiskScore       int      `json:"riskScore"`
	Reasons         []string `json:"reasons"`
	ReviewComment   string   `json:"reviewComment"`
}

type ChatRequest struct {
	UserID  int64  `json:"userId,omitempty"`
	Message string `json:"message"`
}

type ChatResponse struct {
	Intent string      `json:"intent"`
	Answer string      `json:"answer"`
	Data   interface{} `json:"data,omitempty"`
}

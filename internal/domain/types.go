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

package agent

import (
	"context"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"

	"PezMax-Agent/internal/domain"
)

func (s *Service) GenerateStudyPlan(ctx context.Context, req domain.StudyPlanRequest) (domain.StudyPlanResponse, error) {
	req = normalizeStudyPlanRequest(req)
	fileSearch, err := s.SearchFiles(ctx, domain.FileSearchRequest{
		Query:    firstNonEmpty(req.Goal, req.Subject),
		Keyword:  firstNonEmpty(req.Subject, req.Goal),
		School:   req.School,
		Subject:  req.Subject,
		Year:     req.Year,
		PageNum:  1,
		PageSize: 20,
	})
	if err != nil {
		return domain.StudyPlanResponse{}, err
	}

	webSources, webErr := s.searchStudyWebSources(ctx, req)
	materialAnalysis := analyzeStudyMaterials(req, fileSearch.Items, webSources, webErr)
	if s.model != nil {
		aiPlan, err := s.generateStudyPlanWithLLM(ctx, req, fileSearch.Items, webSources, materialAnalysis)
		if err == nil && len(aiPlan.Plan) > 0 {
			aiPlan.Intent = "study_plan"
			aiPlan.Goal = req.Goal
			aiPlan.Subject = req.Subject
			aiPlan.Days = req.Days
			aiPlan.HoursPerDay = req.HoursPerDay
			aiPlan.HasPlatformFiles = len(fileSearch.Items) > 0
			aiPlan.MaterialAnalysis = materialAnalysis
			aiPlan.RecommendedFiles = fileSearch.Results
			aiPlan.WebSources = webSources
			if aiPlan.Summary == "" {
				aiPlan.Summary = summarizeStudyPlan(req, len(fileSearch.Items), len(webSources))
			}
			return aiPlan, nil
		}
	}

	plan := buildStudyPlanDays(req, fileSearch.Items, webSources)
	return domain.StudyPlanResponse{
		Intent:           "study_plan",
		Goal:             req.Goal,
		Subject:          req.Subject,
		Days:             req.Days,
		HoursPerDay:      req.HoursPerDay,
		HasPlatformFiles: len(fileSearch.Items) > 0,
		MaterialAnalysis: materialAnalysis,
		Plan:             plan,
		RecommendedFiles: fileSearch.Results,
		WebSources:       webSources,
		Suggestions:      buildStudyPlanSuggestions(req, fileSearch.Items, webSources, webErr),
		Summary:          summarizeStudyPlan(req, len(fileSearch.Items), len(webSources)),
	}, nil
}

func normalizeStudyPlanRequest(req domain.StudyPlanRequest) domain.StudyPlanRequest {
	req.Goal = strings.TrimSpace(req.Goal)
	if req.Goal == "" {
		req.Goal = strings.TrimSpace(req.Subject)
	}
	if req.Subject == "" {
		req.Subject = extractSubject(req.Goal)
	}
	req.Subject = normalizeSubject(req.Subject)
	if req.Year == 0 {
		req.Year = extractYear(req.Goal)
	}
	if req.Days <= 0 {
		req.Days = extractDays(req.Goal)
	}
	if req.Days <= 0 {
		req.Days = 14
	}
	if req.Days < 1 {
		req.Days = 1
	}
	if req.Days > 60 {
		req.Days = 60
	}
	if req.HoursPerDay <= 0 {
		req.HoursPerDay = extractHoursPerDay(req.Goal)
	}
	if req.HoursPerDay <= 0 {
		req.HoursPerDay = 2
	}
	if req.HoursPerDay < 0.5 {
		req.HoursPerDay = 0.5
	}
	if req.HoursPerDay > 12 {
		req.HoursPerDay = 12
	}
	return req
}

func buildStudyPlanDays(req domain.StudyPlanRequest, files []domain.FileItem, webSources []domain.WebSearchResult) []domain.StudyPlanDay {
	plan := make([]domain.StudyPlanDay, 0, req.Days)
	minutes := int(math.Round(req.HoursPerDay * 60))
	if minutes < 30 {
		minutes = 30
	}
	topics := subjectStudyTopics(req.Subject)

	for day := 1; day <= req.Days; day++ {
		phase, focus := studyPlanPhase(day, req.Days)
		topic := topics[(day-1)%len(topics)]
		tasks := buildStudyTasks(phase, focus, topic, webSources, minutes)
		plan = append(plan, domain.StudyPlanDay{
			Day:              day,
			Title:            fmt.Sprintf("第 %d 天：%s - %s", day, phase, topic.Name),
			Focus:            topic.Focus,
			Tasks:            tasks,
			RecommendedFiles: pickStudyFiles(files, day, 2),
		})
	}
	return plan
}

func studyPlanPhase(day, total int) (string, string) {
	ratio := float64(day) / float64(total)
	switch {
	case ratio <= 0.35:
		return "基础梳理", "回顾核心概念，补齐公式、定义和常见题型"
	case ratio <= 0.7:
		return "专题训练", "围绕高频章节做分题型练习，记录错题原因"
	case ratio <= 0.9:
		return "真题模拟", "按考试节奏完成整套试卷或综合题"
	default:
		return "查漏补缺", "复盘错题与薄弱点，整理考前速记清单"
	}
}

type studyTopic struct {
	Name       string
	Focus      string
	Concepts   string
	Practice   string
	Mistakes   string
	ExamTarget string
}

func subjectStudyTopics(subject string) []studyTopic {
	switch normalizeSubject(subject) {
	case "高等数学":
		return []studyTopic{
			{Name: "函数、极限与连续", Focus: "掌握极限存在、无穷小等价替换、连续与间断点判定", Concepts: "整理常用等价无穷小、洛必达条件、左右极限和连续性判定步骤。", Practice: "完成极限计算、间断点分类、连续性证明各 3-5 题。", Mistakes: "标记等价替换使用条件、洛必达前提和分段函数左右极限错误。", ExamTarget: "能够在 8 分钟内完成 2 道常规极限题。"},
			{Name: "导数与微分", Focus: "强化求导法则、隐函数求导、参数方程求导和高阶导数", Concepts: "复盘复合函数、隐函数、参数方程、反函数求导公式。", Practice: "做求导与切线法线题，加入 2 道隐函数或参数方程综合题。", Mistakes: "记录链式法则漏乘、隐函数移项和高阶导数符号问题。", ExamTarget: "看到函数形式能快速判断求导路径。"},
			{Name: "中值定理与导数应用", Focus: "掌握罗尔、拉格朗日、柯西中值定理和单调性极值", Concepts: "梳理三大中值定理条件，整理单调区间、极值、凹凸性步骤。", Practice: "完成证明题 2 道、单调极值与最值题 4 道。", Mistakes: "检查闭区间连续、开区间可导、端点值相等等条件是否漏写。", ExamTarget: "证明题能先写条件再选定理。"},
			{Name: "不定积分", Focus: "熟练换元、分部积分、有理函数积分和三角代换", Concepts: "整理第一/第二换元、分部积分选 u 原则和常见凑微分。", Practice: "按方法各做 3 题，重点练分部积分和凑微分。", Mistakes: "记录换元后上下限/回代、分部积分符号和常数 C 遗漏。", ExamTarget: "能根据被积函数结构选择积分方法。"},
			{Name: "定积分及应用", Focus: "掌握定积分性质、变限积分、面积体积和物理应用", Concepts: "复盘奇偶性、周期性、变限积分求导、几何应用公式。", Practice: "完成变限积分 3 题、面积体积应用 3 题。", Mistakes: "检查积分区间、旋转轴、绝对值面积和变量替换。", ExamTarget: "应用题能先画区间再列积分。"},
			{Name: "多元函数微分", Focus: "掌握偏导、全微分、复合函数求导和条件极值", Concepts: "整理链式法则树、梯度、二元极值和拉格朗日乘数法。", Practice: "完成偏导/全微分 4 题、条件极值 2 题。", Mistakes: "标记变量依赖关系、二阶偏导顺序和约束方程遗漏。", ExamTarget: "能把复合关系画成求导链。"},
			{Name: "重积分与级数", Focus: "复习二重积分换序、极坐标、数项级数和幂级数", Concepts: "整理积分区域描述、极坐标雅可比、级数判别法。", Practice: "做二重积分 4 题、级数判敛与收敛域 4 题。", Mistakes: "记录换序边界、极坐标 r、端点收敛性漏判。", ExamTarget: "能准确画出二重积分区域。"},
		}
	case "线性代数":
		return []studyTopic{
			{Name: "行列式", Focus: "掌握行列式性质、展开、化三角和递推行列式", Concepts: "整理行列式按行列展开、倍加性质、范德蒙德常见形态。", Practice: "完成化三角计算 4 题、含参数行列式 2 题。", Mistakes: "记录交换行变号、提取公因子和倍加不变的细节。", ExamTarget: "优先用初等变换把行列式化简。"},
			{Name: "矩阵运算与逆矩阵", Focus: "强化矩阵乘法、初等变换、逆矩阵和分块矩阵", Concepts: "复盘可逆条件、伴随矩阵、初等矩阵和矩阵方程。", Practice: "做求逆、矩阵方程、分块矩阵各 2-3 题。", Mistakes: "检查矩阵乘法顺序和初等变换作用对象。", ExamTarget: "能快速判断是否可逆并选择求逆方法。"},
			{Name: "向量组与秩", Focus: "掌握线性相关、极大无关组、秩和等价关系", Concepts: "整理向量组相关性判定、秩的性质和齐次方程联系。", Practice: "完成求秩、极大无关组、线性表示各 3 题。", Mistakes: "记录行变换后向量位置和参数讨论遗漏。", ExamTarget: "把向量组问题转为矩阵秩问题。"},
			{Name: "线性方程组", Focus: "掌握解的判定、基础解系、通解和参数方程组", Concepts: "复盘 r(A) 与 r(A|b)、自由变量和基础解系构造。", Practice: "做齐次/非齐次方程组各 3 题，含参数讨论 2 题。", Mistakes: "检查自由变量个数、特解和通解表达。", ExamTarget: "能用秩快速判断解的情况。"},
			{Name: "特征值与相似对角化", Focus: "掌握特征值、特征向量、相似矩阵和对角化条件", Concepts: "整理特征多项式、代数重数、几何重数和实对称矩阵性质。", Practice: "完成求特征值向量 3 题、对角化 3 题。", Mistakes: "记录重根对应特征向量个数和 P 矩阵列顺序。", ExamTarget: "能判断矩阵是否可对角化。"},
			{Name: "二次型", Focus: "掌握合同变换、标准形、正定判别和惯性指数", Concepts: "整理配方法、正交变换、顺序主子式判别。", Practice: "完成化标准形 4 题、正定判别 3 题。", Mistakes: "检查合同与相似概念混淆。", ExamTarget: "能选择配方法或特征值法化二次型。"},
		}
	case "数据结构":
		return []studyTopic{
			{Name: "线性表", Focus: "掌握顺序表、链表、插入删除和复杂度分析", Concepts: "整理顺序存储与链式存储差异、头插尾插和边界条件。", Practice: "手写链表插入、删除、逆置、合并有序表算法。", Mistakes: "记录空链表、单节点、尾指针和指针断链问题。", ExamTarget: "能写出关键代码并说明时间复杂度。"},
			{Name: "栈、队列与递归", Focus: "掌握栈队列操作、循环队列、表达式求值和递归转非递归", Concepts: "复盘栈顶/队头指针、循环队列判空判满、递归调用栈。", Practice: "完成括号匹配、表达式转换、循环队列入出队题。", Mistakes: "检查 front/rear 更新和取模边界。", ExamTarget: "能解释算法执行过程和状态变化。"},
			{Name: "树与二叉树", Focus: "掌握遍历、线索二叉树、哈夫曼树和树森林转换", Concepts: "整理先中后层序遍历、递归/非递归遍历和哈夫曼编码。", Practice: "根据遍历序列构造二叉树，手写遍历算法。", Mistakes: "记录中序定位、空子树和哈夫曼权值合并错误。", ExamTarget: "能由遍历序列还原树结构。"},
			{Name: "图", Focus: "掌握存储结构、DFS/BFS、最小生成树和最短路径", Concepts: "复盘邻接矩阵/表、Prim、Kruskal、Dijkstra、Floyd。", Practice: "手算遍历序列、最小生成树和单源最短路径。", Mistakes: "记录访问标记、边权更新和路径松弛顺序。", ExamTarget: "能按表格写出算法每轮状态。"},
			{Name: "查找", Focus: "掌握顺序/折半查找、BST、AVL、散列表和平均查找长度", Concepts: "整理 ASL 计算、二叉排序树插删、哈希冲突处理。", Practice: "完成折半查找判定树、哈希表构造和 ASL 计算。", Mistakes: "检查比较次数、装填因子和旋转类型。", ExamTarget: "能准确计算查找成功/失败 ASL。"},
			{Name: "排序", Focus: "掌握插入、交换、选择、归并、基数排序和复杂度稳定性", Concepts: "整理每种排序的时间复杂度、空间复杂度、稳定性和适用场景。", Practice: "手推快速排序、堆排序、归并排序过程。", Mistakes: "记录一趟排序结果、堆调整方向和稳定性判断。", ExamTarget: "能通过中间序列判断排序算法。"},
		}
	case "计算机网络":
		return []studyTopic{
			{Name: "网络体系结构", Focus: "掌握 OSI/TCP-IP 分层、协议数据单元和封装过程", Concepts: "整理各层功能、典型协议和地址/端口/报文关系。", Practice: "完成分层判断、协议归属和封装解封装题。", Mistakes: "记录层次混淆和设备工作层判断错误。", ExamTarget: "能把一次访问网页过程按层解释。"},
			{Name: "物理层与数据链路层", Focus: "掌握编码、差错控制、MAC、CSMA/CD 和以太网帧", Concepts: "复盘带宽、码元、CRC、滑动窗口和交换机转发。", Practice: "做 CRC、帧格式、介质访问控制计算题。", Mistakes: "检查单位换算、生成多项式和窗口大小。", ExamTarget: "能完成常见链路层计算。"},
			{Name: "网络层", Focus: "掌握 IP 地址、子网划分、路由选择和 ICMP", Concepts: "整理 CIDR、子网掩码、最长前缀匹配、ARP 与 ICMP。", Practice: "完成子网划分、路由表匹配、IP 分片题。", Mistakes: "记录网络号广播号、可用主机数和分片偏移。", ExamTarget: "能手算子网和路由匹配。"},
			{Name: "传输层", Focus: "掌握 TCP/UDP、可靠传输、拥塞控制和三次握手四次挥手", Concepts: "整理序号确认号、滑动窗口、慢开始、拥塞避免。", Practice: "做 TCP 状态、确认号、吞吐量和窗口计算题。", Mistakes: "检查 ACK 含义、超时重传和连接释放状态。", ExamTarget: "能画出 TCP 连接过程。"},
			{Name: "应用层", Focus: "掌握 DNS、HTTP、SMTP、FTP 和常见端口", Concepts: "复盘 DNS 递归/迭代查询、HTTP 报文和缓存。", Practice: "完成 URL 访问流程、DNS 查询、HTTP 状态码题。", Mistakes: "记录协议端口和传输层协议对应关系。", ExamTarget: "能解释浏览器访问网站全过程。"},
		}
	case "操作系统":
		return []studyTopic{
			{Name: "进程与线程", Focus: "掌握进程状态、PCB、线程模型和上下文切换", Concepts: "整理进程状态转换、临界区、同步互斥基本概念。", Practice: "完成状态转换、PV 操作和同步关系题。", Mistakes: "记录 wait/signal 顺序和信号量初值错误。", ExamTarget: "能从题意抽象同步互斥关系。"},
			{Name: "处理机调度", Focus: "掌握 FCFS、SJF、优先级、RR 和多级反馈队列", Concepts: "复盘周转时间、带权周转时间、响应时间计算。", Practice: "手算不同调度算法甘特图和平均指标。", Mistakes: "检查到达时间、抢占时机和时间片轮转。", ExamTarget: "能稳定画出调度甘特图。"},
			{Name: "死锁", Focus: "掌握死锁条件、预防避免检测和银行家算法", Concepts: "整理安全序列、资源分配图和死锁必要条件。", Practice: "完成银行家算法安全性判断和资源请求题。", Mistakes: "记录 Need 矩阵、Available 更新和安全序列遗漏。", ExamTarget: "能独立判断系统是否安全。"},
			{Name: "内存管理", Focus: "掌握分页、分段、虚拟内存、页面置换算法", Concepts: "整理页表、快表、地址转换、FIFO/LRU/OPT。", Practice: "完成地址转换、缺页次数和有效访问时间计算。", Mistakes: "检查页号页内偏移、页框号和置换顺序。", ExamTarget: "能手算分页地址转换。"},
			{Name: "文件与 I/O", Focus: "掌握文件组织、目录、磁盘调度和设备管理", Concepts: "复盘连续/链接/索引分配、SCAN/C-SCAN 调度。", Practice: "完成磁盘调度移动道数、索引分配和目录题。", Mistakes: "记录磁头方向和索引块层级。", ExamTarget: "能比较不同文件分配方式。"},
		}
	case "数据库":
		return []studyTopic{
			{Name: "关系模型与 SQL 基础", Focus: "掌握关系代数、SELECT、连接、聚合和分组", Concepts: "整理投影选择连接、GROUP BY、HAVING、子查询。", Practice: "写 8-10 条查询语句，覆盖连接、分组、嵌套查询。", Mistakes: "记录 WHERE/HAVING 区别和外连接空值处理。", ExamTarget: "能把题目文字转成 SQL。"},
			{Name: "数据库设计", Focus: "掌握 ER 图、函数依赖、范式和模式分解", Concepts: "整理实体联系、候选码、1NF/2NF/3NF/BCNF。", Practice: "完成函数依赖闭包、候选码、范式判定和分解题。", Mistakes: "记录部分依赖、传递依赖和无损连接判断。", ExamTarget: "能说明设计是否满足范式。"},
			{Name: "事务与并发控制", Focus: "掌握 ACID、封锁协议、可串行化和隔离级别", Concepts: "复盘读写冲突、两段锁、死锁和 MVCC 基本思路。", Practice: "完成调度冲突可串行化、封锁协议判断题。", Mistakes: "记录脏读、不可重复读、幻读区别。", ExamTarget: "能判断并发调度是否正确。"},
			{Name: "恢复与索引", Focus: "掌握日志恢复、检查点、B+ 树索引和查询优化基础", Concepts: "整理 WAL、UNDO/REDO、聚簇/非聚簇索引。", Practice: "做日志恢复流程、B+ 树插入删除和索引选择题。", Mistakes: "记录恢复方向、检查点之后日志处理。", ExamTarget: "能解释索引对查询的影响。"},
		}
	default:
		return []studyTopic{
			{Name: "课程框架梳理", Focus: "梳理课程目录、考试范围和高频知识点", Concepts: "根据教材目录或课件列出章节清单，标记掌握/模糊/不会三类。", Practice: "选择 2-3 个高频章节做基础题，确认薄弱点。", Mistakes: "记录概念混淆、公式记忆和题型识别问题。", ExamTarget: "形成本课程的复习地图。"},
			{Name: "核心概念强化", Focus: "集中突破核心定义、公式、定理或算法", Concepts: "把核心概念整理成一页速记表，补上使用条件。", Practice: "完成对应基础题和变式题，检查是否能独立复述步骤。", Mistakes: "记录使用条件、边界情况和易混点。", ExamTarget: "能说清每个核心知识点解决什么题。"},
			{Name: "题型专项训练", Focus: "按常考题型做专项练习并形成解题模板", Concepts: "整理每类题的识别信号、解题步骤和检查点。", Practice: "每类题至少完成 3 道，优先做近年试卷题。", Mistakes: "记录卡住的第一步和失分点。", ExamTarget: "看到题目能判断所属题型。"},
			{Name: "综合卷训练", Focus: "按考试时间完成整卷或综合练习", Concepts: "复盘整卷知识点分布和分值结构。", Practice: "限时完成一套试卷，结束后立刻订正。", Mistakes: "统计错题所属章节和失分原因。", ExamTarget: "建立时间分配和做题顺序。"},
			{Name: "考前复盘", Focus: "回看错题、速记表和高频题型", Concepts: "压缩为考前一页纸：公式、定理条件、算法步骤或答题模板。", Practice: "重做错题和代表题，确认不再重复出错。", Mistakes: "整理最后 5 个必须避免的错误。", ExamTarget: "形成考前稳定输出清单。"},
		}
	}
}

func buildStudyTasks(phase, fallbackFocus string, topic studyTopic, webSources []domain.WebSearchResult, totalMinutes int) []domain.StudyTask {
	warmup := maxInt(15, totalMinutes/6)
	main := maxInt(30, totalMinutes*3/5)
	review := totalMinutes - warmup - main
	if review < 15 {
		review = 15
		main = totalMinutes - warmup - review
	}

	return []domain.StudyTask{
		{
			Type:    "review",
			Title:   "知识点拆解",
			Detail:  firstNonEmpty(topic.Concepts, fallbackFocus),
			Minutes: warmup,
		},
		{
			Type:    "practice",
			Title:   phase + "训练",
			Detail:  appendWebHint(topic.Practice, webSources),
			Minutes: main,
		},
		{
			Type:    "summary",
			Title:   "错题复盘与达标检查",
			Detail:  fmt.Sprintf("%s 达标要求：%s", topic.Mistakes, firstNonEmpty(topic.ExamTarget, "能独立完成对应题型并说明步骤。")),
			Minutes: review,
		},
	}
}

func pickStudyFiles(files []domain.FileItem, day, limit int) []domain.FileItem {
	if len(files) == 0 || limit <= 0 {
		return nil
	}
	picked := make([]domain.FileItem, 0, limit)
	start := (day - 1) % len(files)
	for offset := 0; offset < len(files) && len(picked) < limit; offset++ {
		picked = append(picked, files[(start+offset)%len(files)])
	}
	return picked
}

func (s *Service) searchStudyWebSources(ctx context.Context, req domain.StudyPlanRequest) ([]domain.WebSearchResult, error) {
	if s.webSearch == nil {
		return nil, fmt.Errorf("web search is not configured")
	}
	querySubject := firstNonEmpty(req.Subject, req.Goal)
	query := strings.TrimSpace(fmt.Sprintf("%s 期末考试 复习重点 真题 课程大纲", querySubject))
	results, err := s.webSearch.Search(ctx, query, 6)
	if err != nil {
		return nil, err
	}
	out := make([]domain.WebSearchResult, 0, len(results))
	for idx, item := range results {
		snippet := item.Snippet
		if idx < 3 {
			if text, err := s.webSearch.FetchText(ctx, item.URL, 1800); err == nil && text != "" {
				snippet = firstNonEmpty(snippet, "") + "\n正文摘录：" + text
			}
		}
		out = append(out, domain.WebSearchResult{
			Title:   item.Title,
			URL:     item.URL,
			Snippet: strings.TrimSpace(snippet),
			Source:  item.Source,
		})
	}
	return out, nil
}

func analyzeStudyMaterials(req domain.StudyPlanRequest, files []domain.FileItem, webSources []domain.WebSearchResult, webErr error) string {
	subject := firstNonEmpty(req.Subject, "该科目")
	if len(files) == 0 {
		if len(webSources) == 0 {
			if webErr != nil {
				return fmt.Sprintf("平台没有检索到%s相关试卷或资料；网络搜索工具不可用：%s。", subject, webErr.Error())
			}
			return fmt.Sprintf("平台没有检索到%s相关试卷或资料，网络侧也暂未返回可用资源。", subject)
		}
		return fmt.Sprintf("平台没有检索到%s相关试卷或资料；已从网络搜索获得 %d 条课程/复习资源线索，计划将以外部考试范围和通用题型训练为主。", subject, len(webSources))
	}

	years := map[int]bool{}
	types := map[string]int{}
	schools := map[string]int{}
	for _, file := range files {
		if file.Year > 0 {
			years[file.Year] = true
		}
		typeName := firstNonEmpty(fileTypeName(file.Type), "资料")
		types[typeName]++
		if file.School != "" {
			schools[file.School]++
		}
	}
	return fmt.Sprintf("平台检索到 %d 份%s相关资料，覆盖 %d 个年份、%d 类资料、%d 所学校；学习计划将优先围绕这些试卷/资料暴露出的考试题型安排练习，并结合 %d 条网络资源补充课程范围。", len(files), subject, len(years), len(types), len(schools), len(webSources))
}

func (s *Service) generateStudyPlanWithLLM(ctx context.Context, req domain.StudyPlanRequest, files []domain.FileItem, webSources []domain.WebSearchResult, analysis string) (domain.StudyPlanResponse, error) {
	prompt := fmt.Sprintf(`请为 PezMax 试题下载平台用户生成学习计划，只返回严格 JSON，不要解释。

用户目标：%s
科目：%s
天数：%d
每天小时：%.1f
平台资料分析：%s
平台资料列表：%s
网络搜索资源：%s
科目参考主题：%s

要求：
1. 如果平台资料列表为空，summary 必须明确说明“平台暂未找到该科目试卷或资料”，但仍结合网络资源给出建议。
2. 计划必须具体到章节、题型、练习方式和复盘标准，不能只写“基础梳理/专题训练”。
3. 每天 tasks 至少 3 项，分别覆盖知识点、试题/题型训练、错题复盘。
4. 输出 JSON 字段：summary, suggestions, plan。plan 每项字段：day,title,focus,tasks。tasks 每项字段：type,title,detail,minutes。
5. 不要编造不存在的平台资料；网络资源只能根据给定 title/snippet/url 归纳。`,
		req.Goal,
		req.Subject,
		req.Days,
		req.HoursPerDay,
		analysis,
		mustJSON(compactStudyFiles(files, 12)),
		mustJSON(compactWebSources(webSources, 6)),
		mustJSON(subjectStudyTopics(req.Subject)),
	)

	msg, err := s.generate(ctx, studySystemPrompt, prompt)
	if err != nil {
		return domain.StudyPlanResponse{}, err
	}
	var out domain.StudyPlanResponse
	if err := decodeJSONObject(msg.Content, &out); err != nil {
		return domain.StudyPlanResponse{}, err
	}
	return out, nil
}

func compactStudyFiles(files []domain.FileItem, limit int) []map[string]interface{} {
	if len(files) > limit {
		files = files[:limit]
	}
	out := make([]map[string]interface{}, 0, len(files))
	for _, file := range files {
		out = append(out, map[string]interface{}{
			"fileId":      file.FileID,
			"fileName":    file.Name,
			"fileSchool":  file.School,
			"fileSubject": file.Subject,
			"fileYear":    file.Year,
			"fileType":    file.Type,
			"remark":      file.Remark,
		})
	}
	return out
}

func compactWebSources(sources []domain.WebSearchResult, limit int) []domain.WebSearchResult {
	if len(sources) > limit {
		return sources[:limit]
	}
	return sources
}

func appendWebHint(detail string, sources []domain.WebSearchResult) string {
	if len(sources) == 0 {
		return detail
	}
	source := sources[0]
	hint := firstNonEmpty(source.Title, source.Snippet)
	if hint == "" {
		return detail
	}
	return detail + " 参考网络线索：" + hint
}

func buildStudyPlanSuggestions(req domain.StudyPlanRequest, files []domain.FileItem, webSources []domain.WebSearchResult, webErr error) []string {
	suggestions := []string{
		"每天结束后把错题按知识点归档，下一天先复盘再做新题。",
		"临近考试前保留至少 2 天做整卷模拟和错题回看。",
	}
	if len(files) == 0 {
		suggestions = append(suggestions, "平台当前没有匹配到该科目的试卷或资料，计划已改为参考网络复习线索和通用考试范围生成。")
	}
	if len(webSources) == 0 && webErr != nil {
		suggestions = append(suggestions, "网络搜索工具暂不可用，可配置 PEZMAX_WEB_SEARCH_PROVIDER 与对应 API Key 后获得外部资料摘要。")
	}
	if req.HoursPerDay < 1.5 {
		suggestions = append(suggestions, "每日时间较短，建议优先做高频题型和近年真题。")
	}
	return suggestions
}

func summarizeStudyPlan(req domain.StudyPlanRequest, fileCount, webCount int) string {
	subject := req.Subject
	if subject == "" {
		subject = "当前目标"
	}
	if fileCount == 0 {
		return fmt.Sprintf("平台暂未找到%s相关试卷或资料，已结合 %d 条网络资源线索生成 %d 天学习建议。", subject, webCount, req.Days)
	}
	return fmt.Sprintf("已为%s生成 %d 天学习计划，每天约 %.1f 小时，结合 %d 份平台资料和 %d 条网络资源线索。", subject, req.Days, req.HoursPerDay, fileCount, webCount)
}

func looksLikeStudyPlan(text string) bool {
	return containsAny(text, "学习计划", "复习计划", "备考计划", "学习安排", "复习安排", "规划", "备考")
}

func chatStudyPlanRequest(userID int64, text string) domain.StudyPlanRequest {
	return domain.StudyPlanRequest{
		UserID:      userID,
		Goal:        text,
		Subject:     extractSubject(text),
		Days:        extractDays(text),
		HoursPerDay: extractHoursPerDay(text),
		Year:        extractYear(text),
	}
}

func extractDays(text string) int {
	re := regexp.MustCompile(`(\d+)\s*(个星期|星期|周|个月|月|天|日)`)
	match := re.FindStringSubmatch(text)
	if len(match) >= 3 {
		value, _ := strconv.Atoi(match[1])
		switch {
		case strings.Contains(match[2], "月"):
			return value * 30
		case strings.Contains(match[2], "周") || strings.Contains(match[2], "星期"):
			return value * 7
		default:
			return value
		}
	}

	if strings.Contains(text, "半个月") {
		return 15
	}

	chineseRe := regexp.MustCompile(`([一二两三四五六七八九十]+)\s*(个星期|星期|周|个月|月|天|日)`)
	chineseMatch := chineseRe.FindStringSubmatch(text)
	if len(chineseMatch) >= 3 {
		value := chineseNumberToInt(chineseMatch[1])
		if value <= 0 {
			return 0
		}
		switch {
		case strings.Contains(chineseMatch[2], "月"):
			return value * 30
		case strings.Contains(chineseMatch[2], "周") || strings.Contains(chineseMatch[2], "星期"):
			return value * 7
		default:
			return value
		}
	}
	return 0
}

func chineseNumberToInt(text string) int {
	digits := map[rune]int{
		'一': 1,
		'二': 2,
		'两': 2,
		'三': 3,
		'四': 4,
		'五': 5,
		'六': 6,
		'七': 7,
		'八': 8,
		'九': 9,
	}
	if text == "十" {
		return 10
	}
	if strings.Contains(text, "十") {
		parts := strings.Split(text, "十")
		tens := 1
		if parts[0] != "" {
			for _, r := range parts[0] {
				tens = digits[r]
				break
			}
		}
		ones := 0
		if len(parts) > 1 && parts[1] != "" {
			for _, r := range parts[1] {
				ones = digits[r]
				break
			}
		}
		return tens*10 + ones
	}
	for _, r := range text {
		return digits[r]
	}
	return 0
}

func extractHoursPerDay(text string) float64 {
	re := regexp.MustCompile(`(?:每天|每日)?\s*(\d+(?:\.\d+)?)\s*(小时|h|H)`)
	match := re.FindStringSubmatch(text)
	if len(match) < 2 {
		return 0
	}
	value, _ := strconv.ParseFloat(match[1], 64)
	return value
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

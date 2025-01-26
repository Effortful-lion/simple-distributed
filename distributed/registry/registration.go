package registry

// 服务注册中心

// 注册记录：服务名 + 访问地址
// type Registration struct{
// 	ServiceName ServiceName
// 	ServiceURL  string
// }
type Registration struct {
	ServiceName      ServiceName
	ServiceURL       string
	RequiredServices []ServiceName
	ServiceUpdateURL string		// 更新url，用于自发送依赖服务的更新信息
	HeartbeatURL    string		// 心跳url，用于自发送心跳信息
}

type ServiceName string

// 已经存在的服务：
const (
	LogService     = ServiceName("LogService")
	NewService     = ServiceName("NewService") // TODO:可以删除
	GradingService = ServiceName("GradingService")
	PortalService  = ServiceName("Portal") // Web应用，不是服务
)

// 一个记录
type patchEntry struct {
	Name ServiceName
	URL  string
}

// 增加/删除记录
type patch struct {
	Added   []patchEntry
	Removed []patchEntry
}

### 方案文档
[处理k8s中java应用OOM时的dump文件(非preStop)](http://www.devopser.org/articles/2020/09/17/1600339403553.html)

### 功能：

判断是否存在jvm dump文件"/dumps/oom"，如果存在就把/dumps/oom文件上传至cos，并将告警信息写入普罗，如果不存在则忽略。
普罗会发送企微告警，携带oom文件的下载链接

### 编译：

```
GOOS=linux go build -ldflags="-w -s"
```

### 使用方法：

```
1、添加jvm参数，当应用发生OOM时会自动执行工具"-XX:+HeapDumpOnOutOfMemoryError -XX:HeapDumpPath=/dumps/oom -XX:+ExitOnOutOfMemoryError \ 
-XX:OnOutOfMemoryError=./dump-handler -k \$HOSTNAME -e \$ENV -ka saas -prom 10.150.30.6:9090 -cosurl https://XXXXXX.cos.XXXXXXX.com \
-secret XXXXX -secretkey XXXX"
2、部署应用到k8s时在deployment配置挂载emptyDir的volume目录"/dumps"
```

### 说明：

- PODID
  - k8s pod的id，可以通过$HOSTNAME获取
  - podid命名规范，示例 "ops-demo"，以"-"为分隔符，取一个"ops"值为项目组，后面的"demo"为app名
- ENV
  - 部署环境，可以通过$ENV获取(提前通过deployment配置环境变量ENV到容器中)
- OOM Dump文件路径
  - /dumps/oom


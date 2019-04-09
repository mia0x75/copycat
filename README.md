[![LICENSE](https://img.shields.io/badge/license-Anti%20996-blue.svg)](https://github.com/996icu/996.ICU/blob/master/LICENSE)
[![Badge](https://img.shields.io/badge/link-996.icu-red.svg)](https://996.icu/#/zh_CN)
```
                                                _    
   ___    ___    _ __    _   _    ___    __ _  | |_  
  / __|  / _ \  | '_ \  | | | |  / __|  / _` | | __| 
 | (__  | (_) | | |_) | | |_| | | (__  | (_| | | |_  
  \___|  \___/  | .__/   \__, |  \___|  \__,_|  \__| 
                |_|      |___/                        
```

MariaDB日志事件推送中间件，用于实时计算、实时同步、缓存更新等场景。


支持的协议：
* HTTP协议
* TCP协议

其他协议已经被精简，该中间件主要适用于内网，所以HTTP和TCP已经足够。

在HTTP服务端以及TCP客户端收到消息后，可以更新数据、更新缓存、实时计算、更新全文索引、推送消息等。

代办事项：
* [ ] 日志信息调整
* [x] 配置文件合并
* [ ] 错误信息合并
* [ ] 无用代码删除
* [ ] 项目结构优化

[ec449dbdf83ef949a138225e19090967a8b948c7]
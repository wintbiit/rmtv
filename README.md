# RoboMaster TV
![img.png](docs/img.png)
![img.png](docs/preview.png)

## Deply
### 1. 创建自建应用
https://open.feishu.cn/app

### 2. 开通机器人能力与相关权限
- `im:chat:readonly`
- `im:message`
- `im:resource`

### 3. Kubernetes
```shell
kubectl apply -f https://raw.githubusercontent.com/wintbiit/rmtv/refs/heads/main/kustomization.yaml
```

```yaml
services:
  rmtv:
    image: ghcr.io/wintbiit/rmtv:latest
    restart: always
    volumes:
      - ./data/rmtv:/app/data
    environment:
      COOKIES: "<bilibili_cookies>"
      LARK_APP_ID: "<your_app_id>"
      LARK_APP_SECRET: "<your_app_secret>"
      RMTV_MAX_CARDS_PER_ROUND: "2"
```
填写.

飞书应用机器人每轮在每个群发一张主卡，后续分卡按顺序回复到该卡的话题。`RMTV_MAX_CARDS_PER_ROUND` 仅限制每轮发给每个 Webhook 的卡片数，默认为 2，支持正整数。Webhook 超出的条目仍会入库，最后一张卡会标注未展示数量。

### 4. Run
```bash
docker compose up -d
```
1. 机器人全自动往所有加入群灌水
2. 使用自定义机器人: `volumes`添加`- ./webhooks.txt:/app/webhooks.txt`. url填入webhooks.txt新行，自动识别，无需重启

# artty

AWS SSM 기반 포트포워딩을 한 줄로 띄우기 위한 Go CLI MVP.

EC2를 bastion으로 RDS / 내부 호스트에 로컬 포트로 터널링합니다. SSH 키 / 22번 포트 노출 / Public IP 필요 없음 — AWS 자격증명 + SSM agent + IAM 권한만 있으면 됩니다.

## Prerequisites

- AWS CLI v2 (`aws --version`)
- Session Manager Plugin (`session-manager-plugin --version`)
- 로컬 `~/.aws/credentials` 또는 SSO 등으로 AWS 자격증명 설정
- 대상 EC2: SSM Agent 동작 + IAM role에 `AmazonSSMManagedInstanceCore`
- 사용자 IAM: `ssm:StartSession`, `ssm:TerminateSession`, `ssm:DescribeSessions` 등

## Install (개발 중)

```sh
git clone https://github.com/iamsage9346/artty.git
cd artty
go install
```

`$GOPATH/bin` (또는 `$HOME/go/bin`)이 PATH에 있어야 `artty` 명령으로 실행됩니다.

## Usage

```sh
artty connect \
  --profile default \
  --region ap-northeast-2 \
  --instance-id i-xxxxxxxx \
  --host my-db.xxxxxx.ap-northeast-2.rds.amazonaws.com \
  --remote-port 5432 \
  --local-port 5433
```

내부적으로 다음 AWS CLI를 호출합니다:

```sh
aws ssm start-session \
  --target i-xxxxxxxx \
  --document-name AWS-StartPortForwardingSessionToRemoteHost \
  --parameters '{"host":["..."],"portNumber":["5432"],"localPortNumber":["5433"]}' \
  --region ap-northeast-2 \
  --profile default
```

터널이 살아있는 동안 `localhost:5433`에 접속하면 RDS로 포워딩됩니다. `Ctrl+C`로 종료.

## Flags

| Flag | Default | 설명 |
| --- | --- | --- |
| `--profile` | `default` | AWS profile (`~/.aws/credentials`) |
| `--region` | `ap-northeast-2` | AWS region |
| `--instance-id` | (필수) | bastion EC2 instance ID |
| `--host` | (필수) | 포워딩 대상 호스트 (RDS endpoint 등) |
| `--remote-port` | `5432` | 원격 포트 |
| `--local-port` | `5433` | 로컬 바인딩 포트 |

## Roadmap

- [ ] `~/.artty/config.yaml` 지원 — `artty connect wellcare` 한 단어로 호출
- [ ] AWS profile interactive 선택
- [ ] SSM 가능한 EC2 자동 조회
- [ ] RDS endpoint 자동 조회
- [ ] GitHub Releases 바이너리 배포
- [ ] Homebrew tap (`brew install iamsage9346/artty/artty`)

## License

MIT — see [LICENSE](./LICENSE).

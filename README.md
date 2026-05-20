# artty

AWS SSM Session Manager 기반 RDS 포트포워딩 인터랙티브 CLI.

EC2를 bastion 삼아 비공개 RDS / Aurora 클러스터를 로컬 포트로 터널링합니다. SSH 키 / 22번 포트 노출 / Public IP 모두 불필요 — AWS 자격증명 + SSM agent + IAM 권한만 있으면 됩니다.

## Highlights

- Interactive picker (`charmbracelet/huh`) — AWS profile → region → SSM-eligible EC2 → RDS endpoint → local port
- AWS API로 EC2/RDS 자동 조회 (이름 입력 X)
- `~/.artty/session.json`에 마지막 세션 기억 → 한 번 더 묻고 바로 재연결
- `-H` 옵션으로 실행되는 `aws` CLI 커맨드를 단계마다 미리보기
- 로컬 포트 충돌 자동 감지 + 다음 빈 포트 추천

## Prerequisites

- AWS CLI v2 (`aws --version`)
- Session Manager Plugin (`session-manager-plugin --version`)
- AWS 자격증명 (`~/.aws/credentials`) — `aws configure --profile <name>` 으로 등록
- 대상 EC2: SSM Agent 실행 + IAM role에 `AmazonSSMManagedInstanceCore`
- 사용자 IAM 권한 (최소):
  - `ssm:DescribeInstanceInformation`
  - `ssm:StartSession` (resource: `arn:aws:ssm:*:*:document/AWS-StartPortForwardingSessionToRemoteHost` + 대상 EC2)
  - `ssm:TerminateSession`, `ssm:ResumeSession`
  - `ec2:DescribeInstances`
  - `rds:DescribeDBInstances`, `rds:DescribeDBClusters`

## Install

```sh
git clone https://github.com/iamsage9346/artty.git
cd artty
go install
```

`$HOME/go/bin`이 PATH에 있어야 `artty`를 바로 호출할 수 있습니다. zsh:

```sh
echo 'export PATH="$HOME/go/bin:$PATH"' >> ~/.zprofile
source ~/.zprofile
```

## Usage

```sh
artty            # interactive flow
artty -H         # 단계마다 실행되는 aws 커맨드를 같이 보여줌
artty --history  # alias of -H
```

흐름:

1. (이전 세션이 있으면) `Use last session?` → Yes면 그대로 재연결
2. **AWS Profile** 선택 (`~/.aws/credentials` 목록)
3. **Region** 선택 (profile 기본값 자동 선택)
4. **Bastion EC2** 선택 — SSM-online 인스턴스만 자동 필터링, Name 태그 + AZ 표시
5. **RDS endpoint** 선택 — RDS instance + Aurora cluster (writer) 모두 표시
6. **Local port** 입력 (기본 5433, 점유 중이면 다음 빈 포트 추천)
7. 최종 커맨드 미리보기 + 확인
8. SSM 포트포워딩 세션 개시 (Ctrl+C로 종료)

이후 DB 클라이언트(예: VSCode Database Client, DBeaver, psql)는 `localhost:<local-port>`로 접속.

## Roadmap

- [ ] `~/.artty/config.yaml` 다중 프리셋 (`artty wellcare`처럼 이름으로 호출)
- [ ] 백그라운드 모드 (`-f`)
- [ ] Aurora reader endpoint 선택
- [ ] EC2 SSM 셸 세션 (`artty shell`)
- [ ] GitHub Releases 바이너리 + Homebrew tap (`brew install iamsage9346/artty/artty`)

## License

MIT — see [LICENSE](./LICENSE).

# BG-841: beatoz_x509Verify precompile의 복수 OU 미지원 및 deprecated ECDSA 좌표 접근

## Summary

`beatoz_x509Verify` precompiled contract(`0x…ff00`)는 인증서 체인을 검증한 뒤 subject 인증서의 public key, serial number, OU를 하나의 byte 배열로 반환한다. 이때 OU는 `Subject.OrganizationalUnit[0]` 하나만 길이 정보 없이 결과 끝에 붙인다.

~~~go
if len(subjectCert.Subject.OrganizationalUnit) > 0 {
    result = append(result, []byte(subjectCert.Subject.OrganizationalUnit[0])...)
}
~~~

인증서에 OU가 여러 개 있으면(예: `OU=peer` + `OU=org1`) 첫 번째 OU 외의 값은 모두 유실된다. 또한 "첫 번째" OU가 무엇인지는 인증서의 DER 인코딩 방식에 따라 달라지므로, contract가 기대하는 OU(예: `peer`)가 반환되지 않아 contract 측 검증이 실패한다.

추가로 public key 추출에 `ecdsa.PublicKey`의 `X`, `Y` 필드를 직접 사용하고 있다. 두 필드는 Go 1.25부터 Deprecated이며, `LeftPadBytes(…, 32)`는 32바이트를 초과하는 좌표를 자르지 않아 P-256이 아닌 곡선(P-384 등)의 키가 들어오면 출력 layout 전체가 어긋난다.

## Expected Behavior

- 인증서에 포함된 모든 OU가 손실 없이 반환되어야 한다.
- 반환 byte 배열은 OU 개수와 각 OU의 길이에 관계없이 contract가 모호함 없이 파싱할 수 있어야 한다.
- public key는 표준 API(`crypto/ecdsa`, `crypto/ecdh`)로 직렬화되어야 하며, 지원하지 않는 곡선의 키는 잘못된 layout을 반환하는 대신 error로 거부되어야 한다.

## Current Behavior

### 영향

- 대상: `beatoz_x509Verify` precompile ([ctrlers/vm/evm/precompiled.go](../ctrlers/vm/evm/precompiled.go#L91-L138))
- 증상 1: subject 인증서의 OU가 2개 이상이면 첫 번째 OU만 반환되고 나머지는 유실
- 증상 2: 반환되는 단일 OU가 contract가 기대하는 OU가 아닐 수 있어 contract 측 OU 비교 실패
- 증상 3: P-256 이외 곡선의 키가 들어오면 X, Y가 32바이트를 넘어 serial number와 OU의 offset이 밀린 결과를 error 없이 반환
- 증상 4: 사용 중인 `ecdsa.PublicKey.X`, `ecdsa.PublicKey.Y` 필드가 Deprecated

### 현재 반환 format

| Offset | 길이 | 내용 |
|---|---|---|
| 0 | 32 | public key X (left pad) |
| 32 | 32 | public key Y (left pad) |
| 64 | 32 | serial number (left pad) |
| 96 | 가변 | `OrganizationalUnit[0]` raw bytes, 길이 prefix 없음 |

OU 영역은 "결과의 나머지 전부"로만 정의되므로 OU를 여러 개 붙일 수 있는 구조가 아니다.

### 재현 결과

자체 서명 인증서 1장(체인 길이 1)을 입력으로 `beatoz_x509Verify.Run`을 실행한 결과이다.

| 곡선 | Subject OU | 출력 길이 | `ret[96:]` | 결과 |
|---|---|---:|---|---|
| P-256 | `["peer"]` | 100 | `"peer"` | 정상 |
| P-256 | `["peer", "org1"]` | 100 | `"org1"` | `peer` 유실 |
| P-256 | `["org1", "peer"]` | 100 | `"org1"` | `peer` 유실 |
| P-384 | `["peer"]` | 132 | `00…00 0349 "peer"` | layout 붕괴 |

두 번째, 세 번째 행은 인증서 생성 시 OU 순서와 관계없이 같은 값이 반환됨을 보여준다. Go의 `x509.CreateCertificate`는 같은 Name의 복수 OU를 하나의 multi-valued RDN(`SET OF`)으로 인코딩하고, DER 규칙에 따라 SET 원소를 정렬한다. 따라서 parse 후 `OrganizationalUnit[0]`은 사전순으로 앞선 `"org1"`이 된다. OU를 RDN마다 따로 두는 인증서(Hyperledger Fabric CA 발급 인증서 등)는 RDN 순서가 유지되지만, 어느 경우든 `[0]`이 "의미 있는" OU라는 보장은 없다.

P-384 행에서는 X, Y가 각각 48바이트로 그대로 붙어 offset 96부터 Y의 마지막 32바이트가 시작되고, serial(`0x0349` = 841)과 OU가 그 뒤로 밀려 있다. Contract는 이 값을 정상 결과로 받는다.

### Root cause

#### 1. OU를 첫 번째 원소 하나만, 길이 없이 append

[ctrlers/vm/evm/precompiled.go](../ctrlers/vm/evm/precompiled.go#L133-L135)

~~~go
if len(subjectCert.Subject.OrganizationalUnit) > 0 {
    result = append(result, []byte(subjectCert.Subject.OrganizationalUnit[0])...)
}
~~~

- `OrganizationalUnit`은 `[]string`인데 `[0]`만 사용한다.
- OU 길이를 기록하지 않으므로 여러 OU를 연속으로 붙이면 경계를 구분할 수 없다. 즉, 단순히 반복문으로 바꾸는 것만으로는 해결되지 않고 format 변경이 필요하다.

#### 2. Deprecated 좌표 필드 직접 사용 및 곡선 미검증

[ctrlers/vm/evm/precompiled.go](../ctrlers/vm/evm/precompiled.go#L123-L131)

~~~go
ecdsaKey, ok := subjectCert.PublicKey.(*ecdsa.PublicKey)
...
result = append(result, common.LeftPadBytes(ecdsaKey.X.Bytes(), 32)...)
result = append(result, common.LeftPadBytes(ecdsaKey.Y.Bytes(), 32)...)
~~~

Go 1.25+ `crypto/ecdsa` 문서:

~~~text
X, Y *big.Int
    Deprecated: modifying the raw coordinates can produce invalid keys, and may
    invalidate internal optimizations; moreover, [big.Int] methods are not
    suitable for operating on cryptographic values. To encode and decode
    PublicKey values, use [PublicKey.Bytes] and [ParseUncompressedPublicKey] ...
~~~

- `*ecdsa.PublicKey` 타입만 확인하고 곡선이 P-256인지 확인하지 않는다.
- `common.LeftPadBytes(b, 32)`는 `len(b) >= 32`이면 `b`를 그대로 반환하므로, 48바이트(P-384), 66바이트(P-521) 좌표는 잘리지 않고 그대로 붙는다.

## Your Environment

- 분석 기준 commit: `69266d8` (`Merge pull request #84 from beatoz/fix/bg798`)
- 대상 파일: `ctrlers/vm/evm/precompiled.go`, `ctrlers/vm/evm/precompiled_test.go`
- go.mod: `go 1.23.0`, `toolchain go1.23.1`
- 재현 toolchain: `go1.26.3 darwin/arm64`
- go-ethereum: `v1.13.15`

## Possible fixes

### 1. OU를 길이 prefix 방식으로 모두 반환

각 OU 앞에 1바이트 길이를 두고, 모든 OU를 인증서에 나타난 순서대로 붙인다.

| Offset | 길이 | 내용 |
|---|---|---|
| 0 | 32 | public key X |
| 32 | 32 | public key Y |
| 64 | 32 | serial number (left pad) |
| 96 | 1 | `len(OU1)` |
| 97 | `len(OU1)` | OU1 bytes |
| … | 1 | `len(OU2)` |
| … | `len(OU2)` | OU2 bytes |
| … | … | 이후 OU 반복 |

- OU가 없으면 결과는 96바이트로 끝난다.
- OU 개수는 별도로 기록하지 않고, contract는 offset 96부터 결과 끝까지 `[len][bytes]`를 반복해서 읽는다.
- 1바이트 길이이므로 OU 하나의 최대 길이는 255바이트이다. X.520 `ub-organizational-unit-name`이 64이므로 정상 인증서는 범위 안에 있지만, Go의 parser는 이 상한을 강제하지 않으므로 255바이트를 넘는 OU는 error로 거부해야 한다.
- serial number도 `LeftPadBytes(…, 32)`를 사용하므로 20바이트(RFC 5280 상한)를 넘는 비정상 serial이 들어오면 같은 layout 문제가 생긴다. 32바이트 초과 시 error로 거부하는 검사를 함께 넣는 것을 권장한다.

### 2. public key를 표준 API로 직렬화

두 API 모두 SEC 1 uncompressed point(`0x04 || X || Y`, P-256 기준 65바이트)를 반환한다. 앞의 `0x04`를 제거하면 기존과 동일한 64바이트 `X || Y` layout을 유지할 수 있고, `beatoz_p256Verify`의 입력(x, y) 형식과도 그대로 호환된다.

| 방식 | 최소 Go 버전 | 비고 |
|---|---|---|
| `ecdsaKey.Bytes()` | 1.25 | Deprecated 문서가 직접 권장하는 API. go.mod의 `go 1.23.0` 상향 필요 |
| `ecdsaKey.ECDH()` → `(*ecdh.PublicKey).Bytes()` | 1.20 | 현재 go.mod 그대로 사용 가능 |

두 방식 공통으로 다음을 처리한다.

- `ecdsaKey.Curve != elliptic.P256()`이면 error 반환
- 반환값이 65바이트이고 첫 바이트가 `0x04`인지 확인 후 `[1:]`을 append

### 3. 호환성 및 consensus 검토

- 반환 format이 바뀌므로 `ret[96:]`를 OU 문자열 하나로 읽는 기존 contract 및 SDK는 함께 수정되어야 한다.
- precompile 반환값은 EVM 실행 결과에 직접 반영된다. 이미 운영 중인 chain에서 `0x…ff00`을 호출한 transaction이 있다면, format 변경 후 replay 결과가 달라질 수 있다. 해당 chain에서 호출 이력이 있는지 확인하고, 이력이 있으면 fork height 기반으로 신규 format을 활성화해야 한다.
- `RequiredGas`는 인증서 개수 기준이므로 이번 변경의 영향을 받지 않는다.

### 4. 테스트 보강

[ctrlers/vm/evm/precompiled_test.go](../ctrlers/vm/evm/precompiled_test.go#L186-L206)의 기존 테스트는 `retOU := string(ret[96:])`로 단일 OU를 가정한다. 다음 케이스를 추가한다.

- OU 0개: 결과 96바이트
- OU 1개: `[len][OU]`
- OU 2개 이상 (multi-valued RDN, RDN별 분리 두 경우 모두)
- 255바이트 초과 OU: error
- P-384 등 P-256 이외 곡선 키: error
- 반환된 X, Y가 `ecdsa.PublicKey`와 일치하고 `beatoz_p256Verify` 입력으로 그대로 사용 가능한지 확인

수정 시 검토해야 할 관련 코드 위치:

- OU 직렬화: [ctrlers/vm/evm/precompiled.go](../ctrlers/vm/evm/precompiled.go#L133-L135)
- public key 직렬화: [ctrlers/vm/evm/precompiled.go](../ctrlers/vm/evm/precompiled.go#L123-L131)
- 반환 format 주석: [ctrlers/vm/evm/precompiled.go](../ctrlers/vm/evm/precompiled.go#L86-L90), [ctrlers/vm/evm/precompiled.go](../ctrlers/vm/evm/precompiled.go#L128)
- 기존 테스트의 반환값 파싱: [ctrlers/vm/evm/precompiled_test.go](../ctrlers/vm/evm/precompiled_test.go#L186-L206)

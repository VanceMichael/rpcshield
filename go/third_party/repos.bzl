load("@bazel_gazelle//:deps.bzl", "go_repository")

def go_repositories():
    go_repository(
        name = "cat_dario_mergo",
        importpath = "dario.cat/mergo",
        sum = "h1:AGCNq9Evsj31mOgNPcLyXc+4PNABt905YmuqPYYpBWk=",
        version = "v1.0.0",
    )
    go_repository(
        name = "co_honnef_go_tools",
        importpath = "honnef.co/go/tools",
        sum = "h1:4bH5o3b5ZULQ4UrBmP+63W9r7qIkqJClEA9ko5YKx+I=",
        version = "v0.5.1",
    )
    go_repository(
        name = "com_github_alecthomas_template",
        importpath = "github.com/alecthomas/template",
        sum = "h1:JYp7IbQjafoB+tBA3gMyHYHrpOtNuDiK/uB5uXxq5wM=",
        version = "v0.0.0-20190718012654-fb15b899a751",
    )
    go_repository(
        name = "com_github_alecthomas_units",
        importpath = "github.com/alecthomas/units",
        sum = "h1:UQZhZ2O0vMHr2cI+DC1Mbh0TJxzA3RcLoMsFw+aXw7E=",
        version = "v0.0.0-20190924025748-f65c72e2690d",
    )
    go_repository(
        name = "com_github_anmitsu_go_shlex",
        importpath = "github.com/anmitsu/go-shlex",
        sum = "h1:9AeTilPcZAjCFIImctFaOjnTIavg87rW78vTPkQqLI8=",
        version = "v0.0.0-20200514113438-38f4b401e2be",
    )
    go_repository(
        name = "com_github_apache_thrift",
        importpath = "github.com/apache/thrift",
        sum = "h1:qEy6UW60iVOlUy+b9ZR0d5WzUWYGOo4HfopoyBaNmoY=",
        version = "v0.16.0",
    )
    go_repository(
        name = "com_github_aymanbagabas_go_udiff",
        importpath = "github.com/aymanbagabas/go-udiff",
        sum = "h1:OEIrQ8maEeDBXQDoGCbbTTXYJMYRCRO1fnodZ12Gv5o=",
        version = "v0.4.1",
    )
    go_repository(
        name = "com_github_bazelbuild_rules_go",
        importpath = "github.com/bazelbuild/rules_go",
        sum = "h1:FpVmU5mfoqSiY6dLquUAtldkcKOMLrmNaWrSnXuGPF4=",
        version = "v0.62.0",
    )
    go_repository(
        name = "com_github_beorn7_perks",
        importpath = "github.com/beorn7/perks",
        sum = "h1:VlbKKnNfV8bJzeqoa4cOKqO6bYr3WgKZxO8Z16+hsOM=",
        version = "v1.0.1",
    )
    go_repository(
        name = "com_github_bmizerany_perks",
        importpath = "github.com/bmizerany/perks",
        sum = "h1:AP/Y7sqYicnjGDfD5VcY4CIfh1hRXBUavxrvELjTiOE=",
        version = "v0.0.0-20141205001514-d9a9656a3a4b",
    )
    go_repository(
        name = "com_github_burntsushi_toml",
        importpath = "github.com/BurntSushi/toml",
        sum = "h1:pxW6RcqyfI9/kWtOwnv/G+AzdKuy2ZrqINhenH4HyNs=",
        version = "v1.4.1-0.20240526193622-a339e1f7089c",
    )
    go_repository(
        name = "com_github_bytedance_gopkg",
        importpath = "github.com/bytedance/gopkg",
        sum = "h1:TPBSwH8RsouGCBcMBktLt1AymVo2TVsBVCY4b6TnZ/M=",
        version = "v0.1.3",
    )
    go_repository(
        name = "com_github_bytedance_sonic",
        importpath = "github.com/bytedance/sonic",
        sum = "h1:/PXeWFaR5ElNcVE84U0dOHjiMHQOwNIx3K4ymzh/uSE=",
        version = "v1.15.0",
    )
    go_repository(
        name = "com_github_bytedance_sonic_loader",
        importpath = "github.com/bytedance/sonic/loader",
        sum = "h1:gXH3KVnatgY7loH5/TkeVyXPfESoqSBSBEiDd5VjlgE=",
        version = "v0.5.0",
    )
    go_repository(
        name = "com_github_cactus_go_statsd_client_statsd",
        importpath = "github.com/cactus/go-statsd-client/statsd",
        sum = "h1:bXxS5/Z3/dfc8iFniQfgogNBomo0u+1//9eP+jl8GVo=",
        version = "v0.0.0-20191106001114-12b4e2b38748",
    )
    go_repository(
        name = "com_github_cespare_xxhash_v2",
        importpath = "github.com/cespare/xxhash/v2",
        sum = "h1:UL815xU9SqsFlibzuggzjXhog7bL6oX9BbNZnL2UFvs=",
        version = "v2.3.0",
    )
    go_repository(
        name = "com_github_cloudflare_circl",
        importpath = "github.com/cloudflare/circl",
        sum = "h1:qlCDlTPz2n9fu58M0Nh1J/JzcFpfgkFHHX3O35r5vcU=",
        version = "v1.3.7",
    )
    go_repository(
        name = "com_github_cloudwego_base64x",
        importpath = "github.com/cloudwego/base64x",
        sum = "h1:t11wG9AECkCDk5fMSoxmufanudBtJ+/HemLstXDLI2M=",
        version = "v0.1.6",
    )
    go_repository(
        name = "com_github_cncf_xds_go",
        importpath = "github.com/cncf/xds/go",
        sum = "h1:6xNmx7iTtyBRev0+D/Tv1FZd4SCg8axKApyNyRsAt/w=",
        version = "v0.0.0-20251210132809-ee656c7534f5",
    )
    go_repository(
        name = "com_github_crossdock_crossdock_go",
        importpath = "github.com/crossdock/crossdock-go",
        sum = "h1:WR1qVJzbvrVywhAk4kMQKRPx09AZVI0NdEdYs59iHcA=",
        version = "v0.0.0-20160816171116-049aabb0122b",
    )
    go_repository(
        name = "com_github_cyphar_filepath_securejoin",
        importpath = "github.com/cyphar/filepath-securejoin",
        sum = "h1:Ugdm7cg7i6ZK6x3xDF1oEu1nfkyfH53EtKeQYTC3kyg=",
        version = "v0.2.4",
    )
    go_repository(
        name = "com_github_davecgh_go_spew",
        importpath = "github.com/davecgh/go-spew",
        sum = "h1:vj9j/u1bqnvCEfJOwUhtlOARqs3+rkHYY13jYWTU97c=",
        version = "v1.1.1",
    )
    go_repository(
        name = "com_github_dgryski_go_farm",
        importpath = "github.com/dgryski/go-farm",
        sum = "h1:fAjc9m62+UWV/WAFKLNi6ZS0675eEUC9y3AlwSbQu1Y=",
        version = "v0.0.0-20200201041132-a6ae2369ad13",
    )
    go_repository(
        name = "com_github_emirpasic_gods",
        importpath = "github.com/emirpasic/gods",
        sum = "h1:FXtiHYKDGKCW2KzwZKx0iC0PQmdlorYgdFG9jPXJ1Bc=",
        version = "v1.18.1",
    )
    go_repository(
        name = "com_github_envoyproxy_go_control_plane",
        importpath = "github.com/envoyproxy/go-control-plane",
        sum = "h1:hbG2kr4RuFj222B6+7T83thSPqLjwBIfQawTkC++2HA=",
        version = "v0.14.0",
    )
    go_repository(
        name = "com_github_envoyproxy_go_control_plane_envoy",
        importpath = "github.com/envoyproxy/go-control-plane/envoy",
        sum = "h1:yg/JjO5E7ubRyKX3m07GF3reDNEnfOboJ0QySbH736g=",
        version = "v1.36.0",
    )
    go_repository(
        name = "com_github_envoyproxy_go_control_plane_ratelimit",
        importpath = "github.com/envoyproxy/go-control-plane/ratelimit",
        sum = "h1:/G9QYbddjL25KvtKTv3an9lx6VBE2cnb8wp1vEGNYGI=",
        version = "v0.1.0",
    )
    go_repository(
        name = "com_github_envoyproxy_protoc_gen_validate",
        importpath = "github.com/envoyproxy/protoc-gen-validate",
        sum = "h1:TvGH1wof4H33rezVKWSpqKz5NXWg5VPuZ0uONDT6eb4=",
        version = "v1.3.0",
    )
    go_repository(
        name = "com_github_facebookgo_clock",
        importpath = "github.com/facebookgo/clock",
        sum = "h1:yDWHCSQ40h88yih2JAcL6Ls/kVkSE8GFACTGVnMPruw=",
        version = "v0.0.0-20150410010913-600d898af40a",
    )
    go_repository(
        name = "com_github_fatih_structtag",
        importpath = "github.com/fatih/structtag",
        sum = "h1:/OdNE99OxoI/PqaW/SuSK9uxxT3f/tcSZgon/ssNSx4=",
        version = "v1.2.0",
    )
    go_repository(
        name = "com_github_gabriel_vasile_mimetype",
        importpath = "github.com/gabriel-vasile/mimetype",
        sum = "h1:e9hWvmLYvtp846tLHam2o++qitpguFiYCKbn0w9jyqw=",
        version = "v1.4.12",
    )
    go_repository(
        name = "com_github_gin_contrib_sse",
        importpath = "github.com/gin-contrib/sse",
        sum = "h1:n0w2GMuUpWDVp7qSpvze6fAu9iRxJY4Hmj6AmBOU05w=",
        version = "v1.1.0",
    )
    go_repository(
        name = "com_github_gin_gonic_gin",
        importpath = "github.com/gin-gonic/gin",
        sum = "h1:b3YAbrZtnf8N//yjKeU2+MQsh2mY5htkZidOM7O0wG8=",
        version = "v1.12.0",
    )
    go_repository(
        name = "com_github_go_git_gcfg",
        importpath = "github.com/go-git/gcfg",
        sum = "h1:+zs/tPmkDkHx3U66DAb0lQFJrpS6731Oaa12ikc+DiI=",
        version = "v1.5.1-0.20230307220236-3a3c6141e376",
    )
    go_repository(
        name = "com_github_go_git_go_billy_v5",
        importpath = "github.com/go-git/go-billy/v5",
        sum = "h1:yEY4yhzCDuMGSv83oGxiBotRzhwhNr8VZyphhiu+mTU=",
        version = "v5.5.0",
    )
    go_repository(
        name = "com_github_go_git_go_git_v5",
        importpath = "github.com/go-git/go-git/v5",
        sum = "h1:XIZc1p+8YzypNr34itUfSvYJcv+eYdTnTvOZ2vD3cA4=",
        version = "v5.11.0",
    )
    go_repository(
        name = "com_github_go_jose_go_jose_v4",
        importpath = "github.com/go-jose/go-jose/v4",
        sum = "h1:CVLmWDhDVRa6Mi/IgCgaopNosCaHz7zrMeF9MlZRkrs=",
        version = "v4.1.3",
    )
    go_repository(
        name = "com_github_go_kit_kit",
        importpath = "github.com/go-kit/kit",
        sum = "h1:wDJmvq38kDhkVxi50ni9ykkdUr1PKgqKOoi01fa0Mdk=",
        version = "v0.9.0",
    )
    go_repository(
        name = "com_github_go_kit_log",
        importpath = "github.com/go-kit/log",
        sum = "h1:DGJh0Sm43HbOeYDNnVZFl8BvcYVvjD5bqYJvp0REbwQ=",
        version = "v0.1.0",
    )
    go_repository(
        name = "com_github_go_logfmt_logfmt",
        importpath = "github.com/go-logfmt/logfmt",
        sum = "h1:TrB8swr/68K7m9CcGut2g3UOihhbcbiMAYiuTXdEih4=",
        version = "v0.5.0",
    )
    go_repository(
        name = "com_github_go_logr_logr",
        importpath = "github.com/go-logr/logr",
        sum = "h1:CjnDlHq8ikf6E492q6eKboGOC0T8CDaOvkHCIg8idEI=",
        version = "v1.4.3",
    )
    go_repository(
        name = "com_github_go_logr_stdr",
        importpath = "github.com/go-logr/stdr",
        sum = "h1:hSWxHoqTgW2S2qGc0LTAI563KZ5YKYRhT3MFKZMbjag=",
        version = "v1.2.2",
    )
    go_repository(
        name = "com_github_go_playground_assert_v2",
        importpath = "github.com/go-playground/assert/v2",
        sum = "h1:JvknZsQTYeFEAhQwI4qEt9cyV5ONwRHC+lYKSsYSR8s=",
        version = "v2.2.0",
    )
    go_repository(
        name = "com_github_go_playground_locales",
        importpath = "github.com/go-playground/locales",
        sum = "h1:EWaQ/wswjilfKLTECiXz7Rh+3BjFhfDFKv/oXslEjJA=",
        version = "v0.14.1",
    )
    go_repository(
        name = "com_github_go_playground_universal_translator",
        importpath = "github.com/go-playground/universal-translator",
        sum = "h1:Bcnm0ZwsGyWbCzImXv+pAJnYK9S473LQFuzCbDbfSFY=",
        version = "v0.18.1",
    )
    go_repository(
        name = "com_github_go_playground_validator_v10",
        importpath = "github.com/go-playground/validator/v10",
        sum = "h1:f3zDSN/zOma+w6+1Wswgd9fLkdwy06ntQJp0BBvFG0w=",
        version = "v10.30.1",
    )
    go_repository(
        name = "com_github_go_stack_stack",
        importpath = "github.com/go-stack/stack",
        sum = "h1:5SgMzNM5HxrEjV0ww2lTmX6E2Izsfxas4+YHWRs3Lsk=",
        version = "v1.8.0",
    )
    go_repository(
        name = "com_github_goccy_go_json",
        importpath = "github.com/goccy/go-json",
        sum = "h1:Fq85nIqj+gXn/S5ahsiTlK3TmC85qgirsdTP/+DeaC4=",
        version = "v0.10.5",
    )
    go_repository(
        name = "com_github_goccy_go_yaml",
        importpath = "github.com/goccy/go-yaml",
        sum = "h1:PmFC1S6h8ljIz6gMRBopkjP1TVT7xuwrButHID66PoM=",
        version = "v1.19.2",
    )
    go_repository(
        name = "com_github_gogo_googleapis",
        build_file_proto_mode = "disable",
        importpath = "github.com/gogo/googleapis",
        sum = "h1:1Yx4Myt7BxzvUr5ldGSbwYiZG6t9wGBZ+8/fX3Wvtq0=",
        version = "v1.4.1",
    )
    go_repository(
        name = "com_github_gogo_protobuf",
        importpath = "github.com/gogo/protobuf",
        sum = "h1:Ov1cvc58UF3b5XjBnZv7+opcTcQFZebYjWzi34vdm4Q=",
        version = "v1.3.2",
    )
    go_repository(
        name = "com_github_gogo_status",
        build_file_proto_mode = "disable",
        importpath = "github.com/gogo/status",
        sum = "h1:+eIkrewn5q6b30y+g/BJINVVdi2xH7je5MPJ3ZPK3JA=",
        version = "v1.1.0",
    )
    go_repository(
        name = "com_github_golang_glog",
        importpath = "github.com/golang/glog",
        sum = "h1:DrW6hGnjIhtvhOIiAKT6Psh/Kd/ldepEa81DKeiRJ5I=",
        version = "v1.2.5",
    )
    go_repository(
        name = "com_github_golang_groupcache",
        importpath = "github.com/golang/groupcache",
        sum = "h1:oI5xCqsCo564l8iNU+DwB5epxmsaqB+rhGL0m5jtYqE=",
        version = "v0.0.0-20210331224755-41bb18bfe9da",
    )
    go_repository(
        name = "com_github_golang_jwt_jwt_v5",
        importpath = "github.com/golang-jwt/jwt/v5",
        sum = "h1:Rl4B7itRWVtYIHFrSNd7vhTiz9UpLdi6gZhZ3wEeDy8=",
        version = "v5.2.2",
    )
    go_repository(
        name = "com_github_golang_mock",
        importpath = "github.com/golang/mock",
        sum = "h1:YojYx61/OLFsiv6Rw1Z96LpldJIy31o+UHmwAUMJ6/U=",
        version = "v1.7.0-rc.1",
    )
    go_repository(
        name = "com_github_golang_protobuf",
        importpath = "github.com/golang/protobuf",
        sum = "h1:i7eJL8qZTpSEXOPTxNKhASYpMn+8e5Q6AdndVa1dWek=",
        version = "v1.5.4",
    )
    go_repository(
        name = "com_github_golang_snappy",
        importpath = "github.com/golang/snappy",
        sum = "h1:yAGX7huGHXlcLOEtBnF4w7FQwA26wojNCwOYAEhLjQM=",
        version = "v0.0.4",
    )
    go_repository(
        name = "com_github_google_go_cmp",
        importpath = "github.com/google/go-cmp",
        sum = "h1:wk8382ETsv4JYUZwIsn6YpYiWiBsYLSJiTsyBybVuN8=",
        version = "v0.7.0",
    )
    go_repository(
        name = "com_github_google_gofuzz",
        importpath = "github.com/google/gofuzz",
        sum = "h1:A8PeW59pxE9IoFRqBp37U+mSNaQoZ46F1f0f863XSXw=",
        version = "v1.0.0",
    )
    go_repository(
        name = "com_github_google_renameio",
        importpath = "github.com/google/renameio",
        sum = "h1:GOZbcHa3HfsPKPlmyPyN2KEohoMXOhdMbHrvbpl2QaA=",
        version = "v0.1.0",
    )
    go_repository(
        name = "com_github_google_uuid",
        importpath = "github.com/google/uuid",
        sum = "h1:NIvaJDMOsjHA8n1jAhLSgzrAzy1Hgr+hNrb57e+94F0=",
        version = "v1.6.0",
    )
    go_repository(
        name = "com_github_googlecloudplatform_opentelemetry_operations_go_detectors_gcp",
        importpath = "github.com/GoogleCloudPlatform/opentelemetry-operations-go/detectors/gcp",
        sum = "h1:sBEjpZlNHzK1voKq9695PJSX2o5NEXl7/OL3coiIY0c=",
        version = "v1.30.0",
    )
    go_repository(
        name = "com_github_gorilla_mux",
        importpath = "github.com/gorilla/mux",
        sum = "h1:TuBL49tXwgrFYWhqrNgrUNEY92u81SPhu7sTdzQEiWY=",
        version = "v1.8.1",
    )
    go_repository(
        name = "com_github_hdrhistogram_hdrhistogram_go",
        importpath = "github.com/HdrHistogram/hdrhistogram-go",
        sum = "h1:dpujRju0R4M/QZzcnR1LH1qm+TVG3UzkWdp5tH1WMcg=",
        version = "v0.9.0",
    )
    go_repository(
        name = "com_github_jbenet_go_context",
        importpath = "github.com/jbenet/go-context",
        sum = "h1:BQSFePA1RWJOlocH6Fxy8MmwDt+yVQYULKfN0RoTN8A=",
        version = "v0.0.0-20150711004518-d14ea06fba99",
    )
    go_repository(
        name = "com_github_jessevdk_go_flags",
        importpath = "github.com/jessevdk/go-flags",
        sum = "h1:1jKYvbxEjfUl0fmqTCOfonvskHHXMjBySTLW4y9LFvc=",
        version = "v1.5.0",
    )
    go_repository(
        name = "com_github_jonboulle_clockwork",
        importpath = "github.com/jonboulle/clockwork",
        sum = "h1:p4Cf1aMWXnXAUh8lVfewRBx1zaTSYKrKMF2g3ST4RZ4=",
        version = "v0.4.0",
    )
    go_repository(
        name = "com_github_jordanlewis_gcassert",
        importpath = "github.com/jordanlewis/gcassert",
        sum = "h1:a+PGEeXb+exwBS3NboqXHyxarD9kaboBbrSp+7GuBuc=",
        version = "v0.0.0-20250430164644-389ef753e22e",
    )
    go_repository(
        name = "com_github_jpillora_backoff",
        importpath = "github.com/jpillora/backoff",
        sum = "h1:uvFg412JmmHBHw7iwprIxkPMI+sGQ4kzOWsMeHnm2EA=",
        version = "v1.0.0",
    )
    go_repository(
        name = "com_github_json_iterator_go",
        importpath = "github.com/json-iterator/go",
        sum = "h1:PV8peI4a0ysnczrg+LtxykD8LfKY9ML6u2jnxaEnrnM=",
        version = "v1.1.12",
    )
    go_repository(
        name = "com_github_julienschmidt_httprouter",
        importpath = "github.com/julienschmidt/httprouter",
        sum = "h1:U0609e9tgbseu3rBINet9P48AI/D3oJs4dN7jwJOQ1U=",
        version = "v1.3.0",
    )
    go_repository(
        name = "com_github_kevinburke_ssh_config",
        importpath = "github.com/kevinburke/ssh_config",
        sum = "h1:x584FjTGwHzMwvHx18PXxbBVzfnxogHaAReU4gf13a4=",
        version = "v1.2.0",
    )
    go_repository(
        name = "com_github_kisielk_errcheck",
        importpath = "github.com/kisielk/errcheck",
        sum = "h1:+SbscKmWJ5mOK/bO1zS60F5I9WwZDWOfRsC4RwfwRV0=",
        version = "v1.7.0",
    )
    go_repository(
        name = "com_github_kisielk_gotool",
        importpath = "github.com/kisielk/gotool",
        sum = "h1:AV2c/EiW3KqPNT9ZKl07ehoAGi4C5/01Cfbblndcapg=",
        version = "v1.0.0",
    )
    go_repository(
        name = "com_github_klauspost_compress",
        importpath = "github.com/klauspost/compress",
        sum = "h1:RPhnKRAQ4Fh8zU2FY/6ZFDwTVTxgJ/EMydqSTzE9a2c=",
        version = "v1.18.4",
    )
    go_repository(
        name = "com_github_klauspost_cpuid_v2",
        importpath = "github.com/klauspost/cpuid/v2",
        sum = "h1:S4CRMLnYUhGeDFDqkGriYKdfoFlDnMtqTiI/sFzhA9Y=",
        version = "v2.3.0",
    )
    go_repository(
        name = "com_github_konsorten_go_windows_terminal_sequences",
        importpath = "github.com/konsorten/go-windows-terminal-sequences",
        sum = "h1:CE8S1cTafDpPvMhIxNJKvHsGVBgn1xWYf1NbHQhywc8=",
        version = "v1.0.3",
    )
    go_repository(
        name = "com_github_kr_logfmt",
        importpath = "github.com/kr/logfmt",
        sum = "h1:T+h1c/A9Gawja4Y9mFVWj2vyii2bbUNDw3kt9VxK2EY=",
        version = "v0.0.0-20140226030751-b84e30acd515",
    )
    go_repository(
        name = "com_github_kr_pretty",
        importpath = "github.com/kr/pretty",
        sum = "h1:flRD4NNwYAUpkphVc1HcthR4KEIFJ65n8Mw5qdRn3LE=",
        version = "v0.3.1",
    )
    go_repository(
        name = "com_github_kr_pty",
        importpath = "github.com/kr/pty",
        sum = "h1:VkoXIwSboBpnk99O/KFauAEILuNHv5DVFKZMBN/gUgw=",
        version = "v1.1.1",
    )
    go_repository(
        name = "com_github_kr_text",
        importpath = "github.com/kr/text",
        sum = "h1:5Nx0Ya0ZqY2ygV366QzturHI13Jq95ApcVaJBhpS+AY=",
        version = "v0.2.0",
    )
    go_repository(
        name = "com_github_leodido_go_urn",
        importpath = "github.com/leodido/go-urn",
        sum = "h1:WT9HwE9SGECu3lg4d/dIA+jxlljEa1/ffXKmRjqdmIQ=",
        version = "v1.4.0",
    )
    go_repository(
        name = "com_github_marusama_semaphore_v2",
        importpath = "github.com/marusama/semaphore/v2",
        sum = "h1:o/1QJD9DBYOWRnDhPwDVAXQn6mQYD0gZaS1Tpx6DJGM=",
        version = "v2.5.0",
    )
    go_repository(
        name = "com_github_mattn_go_isatty",
        importpath = "github.com/mattn/go-isatty",
        sum = "h1:xfD0iDuEKnDkl03q4limB+vH+GxLEtL/jb4xVJSWWEY=",
        version = "v0.0.20",
    )
    go_repository(
        name = "com_github_mattn_go_shellwords",
        importpath = "github.com/mattn/go-shellwords",
        sum = "h1:M2zGm7EW6UQJvDeQxo4T51eKPurbeFbe8WtebGE2xrk=",
        version = "v1.0.12",
    )
    go_repository(
        name = "com_github_matttproud_golang_protobuf_extensions",
        importpath = "github.com/matttproud/golang_protobuf_extensions",
        sum = "h1:I0XW9+e1XWDxdcEniV4rQAIOPUGDq67JSCiRCgGCZLI=",
        version = "v1.0.2-0.20181231171920-c182affec369",
    )
    go_repository(
        name = "com_github_microsoft_go_winio",
        importpath = "github.com/Microsoft/go-winio",
        sum = "h1:9/kr64B9VUZrLm5YYwbGtUJnMgqWVOdUAXu6Migciow=",
        version = "v0.6.1",
    )
    go_repository(
        name = "com_github_modern_go_concurrent",
        importpath = "github.com/modern-go/concurrent",
        sum = "h1:TRLaZ9cD/w8PVh93nsPXa1VrQ6jlwL5oN8l14QlcNfg=",
        version = "v0.0.0-20180306012644-bacd9c7ef1dd",
    )
    go_repository(
        name = "com_github_modern_go_reflect2",
        importpath = "github.com/modern-go/reflect2",
        sum = "h1:xBagoLtFs94CBntxluKeaWgTMpvLxC4ur3nMaC9Gz0M=",
        version = "v1.0.2",
    )
    go_repository(
        name = "com_github_mwitkow_go_conntrack",
        importpath = "github.com/mwitkow/go-conntrack",
        sum = "h1:KUppIJq7/+SVif2QVs3tOP0zanoHgBEVAwHxUSIzRqU=",
        version = "v0.0.0-20190716064945-2f068394615f",
    )
    go_repository(
        name = "com_github_niemeyer_pretty",
        importpath = "github.com/niemeyer/pretty",
        sum = "h1:fD57ERR4JtEqsWbfPhv4DMiApHyliiK5xCTNVSPiaAs=",
        version = "v0.0.0-20200227124842-a10e7caefd8e",
    )
    go_repository(
        name = "com_github_opentracing_opentracing_go",
        importpath = "github.com/opentracing/opentracing-go",
        sum = "h1:uEJPy/1a5RIPAJ0Ov+OIO8OxWu77jEv+1B0VhjKrZUs=",
        version = "v1.2.0",
    )
    go_repository(
        name = "com_github_pborman_uuid",
        importpath = "github.com/pborman/uuid",
        sum = "h1:l8VQbMdmwFH37kOOaWQ/cw24/u8AuBz5lUym13Wcu0Y=",
        version = "v0.0.0-20160209185913-a97ce2ca70fa",
    )
    go_repository(
        name = "com_github_pelletier_go_toml_v2",
        importpath = "github.com/pelletier/go-toml/v2",
        sum = "h1:mye9XuhQ6gvn5h28+VilKrrPoQVanw5PMw/TB0t5Ec4=",
        version = "v2.2.4",
    )
    go_repository(
        name = "com_github_pjbgf_sha1cd",
        importpath = "github.com/pjbgf/sha1cd",
        sum = "h1:4D5XXmUUBUl/xQ6IjCkEAbqXskkq/4O7LmGn0AqMDs4=",
        version = "v0.3.0",
    )
    go_repository(
        name = "com_github_pkg_errors",
        importpath = "github.com/pkg/errors",
        sum = "h1:FEBLx1zS214owpjy7qsBeixbURkuhQAwrK5UwLGTwt4=",
        version = "v0.9.1",
    )
    go_repository(
        name = "com_github_planetscale_vtprotobuf",
        importpath = "github.com/planetscale/vtprotobuf",
        sum = "h1:GFCKgmp0tecUJ0sJuv4pzYCqS9+RGSn52M3FUwPs+uo=",
        version = "v0.6.1-0.20240319094008-0393e58bdf10",
    )
    go_repository(
        name = "com_github_pmezard_go_difflib",
        importpath = "github.com/pmezard/go-difflib",
        sum = "h1:4DBwDE0NGyQoBHbLQYPwSUPoCMWR5BEzIk/f1lZbAQM=",
        version = "v1.0.0",
    )
    go_repository(
        name = "com_github_prashantv_protectmem",
        importpath = "github.com/prashantv/protectmem",
        sum = "h1:AA9vgIBDjMHPC2McaGPojgV2dcI78ZC0TLNhYCXEKH8=",
        version = "v0.0.0-20171002184600-e20412882b3a",
    )
    go_repository(
        name = "com_github_prometheus_client_golang",
        importpath = "github.com/prometheus/client_golang",
        sum = "h1:+4eQaD7vAZ6DsfsxB15hbE0odUjGI5ARs9yskGu1v4s=",
        version = "v1.11.1",
    )
    go_repository(
        name = "com_github_prometheus_client_model",
        importpath = "github.com/prometheus/client_model",
        sum = "h1:uq5h0d+GuxiXLJLNABMgp2qUWDPiLvgCzz2dUR+/W/M=",
        version = "v0.2.0",
    )
    go_repository(
        name = "com_github_prometheus_common",
        importpath = "github.com/prometheus/common",
        sum = "h1:iMAkS2TDoNWnKM+Kopnx/8tnEStIfpYA0ur0xQzzhMQ=",
        version = "v0.26.0",
    )
    go_repository(
        name = "com_github_prometheus_procfs",
        importpath = "github.com/prometheus/procfs",
        sum = "h1:mxy4L2jP6qMonqmq+aTtOx1ifVWUgG/TAmntgbh3xv4=",
        version = "v0.6.0",
    )
    go_repository(
        name = "com_github_protonmail_go_crypto",
        importpath = "github.com/ProtonMail/go-crypto",
        sum = "h1:kkhsdkhsCvIsutKu5zLMgWtgh9YxGCNAw8Ad8hjwfYg=",
        version = "v0.0.0-20230828082145-3c4c8a2d2371",
    )
    go_repository(
        name = "com_github_puzpuzpuz_xsync_v4",
        importpath = "github.com/puzpuzpuz/xsync/v4",
        sum = "h1:vOSWu6b57/emh+L/Cw0BeQfvxa/cogFywXHeGUxQxAg=",
        version = "v4.5.0",
    )
    go_repository(
        name = "com_github_quic_go_qpack",
        importpath = "github.com/quic-go/qpack",
        sum = "h1:g7W+BMYynC1LbYLSqRt8PBg5Tgwxn214ZZR34VIOjz8=",
        version = "v0.6.0",
    )
    go_repository(
        name = "com_github_quic_go_quic_go",
        importpath = "github.com/quic-go/quic-go",
        sum = "h1:0Gmua0HW1Tv7ANR7hUYwRyD0MG5OJfgvYSZasGZzBic=",
        version = "v0.59.1",
    )
    go_repository(
        name = "com_github_robfig_cron",
        importpath = "github.com/robfig/cron",
        sum = "h1:ZjScXvvxeQ63Dbyxy76Fj3AT3Ut0aKsyd2/tl3DTMuQ=",
        version = "v1.2.0",
    )
    go_repository(
        name = "com_github_rogpeppe_go_internal",
        importpath = "github.com/rogpeppe/go-internal",
        sum = "h1:cWPaGQEPrBb5/AsnsZesgZZ9yb1OQ+GOISoDNXVBh4M=",
        version = "v1.11.0",
    )
    go_repository(
        name = "com_github_samuel_go_thrift",
        importpath = "github.com/samuel/go-thrift",
        sum = "h1:EiWVfh8mr40yFZEui2oF0d45KgH48PkB2H0Z0GANvSI=",
        version = "v0.0.0-20191111193933-5165175b40af",
    )
    go_repository(
        name = "com_github_sashabaranov_go_openai",
        importpath = "github.com/sashabaranov/go-openai",
        sum = "h1:MPcAwlwbeo7ZmhQczoOgZBHtIBY1TfZqsdx6+/ndloM=",
        version = "v1.17.7",
    )
    go_repository(
        name = "com_github_sergi_go_diff",
        importpath = "github.com/sergi/go-diff",
        sum = "h1:we8PVUC3FE2uYfodKH/nBHMSetSfHDR6scGdBi+erh0=",
        version = "v1.1.0",
    )
    go_repository(
        name = "com_github_sirupsen_logrus",
        importpath = "github.com/sirupsen/logrus",
        sum = "h1:UBcNElsrwanuuMsnGSlYmtmgbb23qDR5dG+6X6Oo89I=",
        version = "v1.6.0",
    )
    go_repository(
        name = "com_github_skeema_knownhosts",
        importpath = "github.com/skeema/knownhosts",
        sum = "h1:SHWdIUa82uGZz+F+47k8SY4QhhI291cXCpopT1lK2AQ=",
        version = "v1.2.1",
    )
    go_repository(
        name = "com_github_smacker_go_tree_sitter",
        importpath = "github.com/smacker/go-tree-sitter",
        sum = "h1:6C8qej6f1bStuePVkLSFxoU22XBS165D3klxlzRg8F4=",
        version = "v0.0.0-20240827094217-dd81d9e9be82",
    )
    go_repository(
        name = "com_github_spiffe_go_spiffe_v2",
        importpath = "github.com/spiffe/go-spiffe/v2",
        sum = "h1:l+DolpxNWYgruGQVV0xsfeya3CsC7m8iBzDnMpsbLuo=",
        version = "v2.6.0",
    )
    go_repository(
        name = "com_github_streadway_quantile",
        importpath = "github.com/streadway/quantile",
        sum = "h1:X4+kt6zM/OVO6gbJdAfJR60MGPsqCzbtXNnjoGqdfAs=",
        version = "v0.0.0-20220407130108-4246515d968d",
    )
    go_repository(
        name = "com_github_stretchr_objx",
        importpath = "github.com/stretchr/objx",
        sum = "h1:xuMeJ0Sdp5ZMRXx/aWO6RZxdr3beISkG5/G/aIRr3pY=",
        version = "v0.5.2",
    )
    go_repository(
        name = "com_github_stretchr_testify",
        importpath = "github.com/stretchr/testify",
        sum = "h1:7s2iGBzp5EwR7/aIZr8ao5+dra3wiQyKjjFuvgVKu7U=",
        version = "v1.11.1",
    )
    go_repository(
        name = "com_github_twitchyliquid64_golang_asm",
        importpath = "github.com/twitchyliquid64/golang-asm",
        sum = "h1:SU5vSMR7hnwNxj24w34ZyCi/FmDZTkS4MhqMhdFk5YI=",
        version = "v0.15.1",
    )
    go_repository(
        name = "com_github_twmb_murmur3",
        importpath = "github.com/twmb/murmur3",
        sum = "h1:8Yt9taO/WN3l08xErzjeschgZU2QSrwm1kclYq+0aRg=",
        version = "v1.1.8",
    )
    go_repository(
        name = "com_github_uber_cadence_idl",
        importpath = "github.com/uber/cadence-idl",
        sum = "h1:YJlEu9Unzifwdn6SuE4rrl4zJ5lop5gBfSX8AyodTww=",
        version = "v0.0.0-20250616185004-cc6f52f87bc6",
    )
    go_repository(
        name = "com_github_uber_common_bark",
        importpath = "github.com/uber-common/bark",
        sum = "h1:cREJ9b7CpTjwZr0/5wV82fXlitoCIEHHnt9WkQ4lIk0=",
        version = "v1.2.1",
    )
    go_repository(
        name = "com_github_uber_go_mapdecode",
        importpath = "github.com/uber-go/mapdecode",
        sum = "h1:euUEFM9KnuCa1OBixz1xM+FIXmpixyay5DLymceOVrU=",
        version = "v1.0.0",
    )
    go_repository(
        name = "com_github_uber_go_tally",
        importpath = "github.com/uber-go/tally",
        sum = "h1:Z2vK6ib6G/r6bAGu7lAI/98cLPLUOtdHrY2bBikk4wg=",
        version = "v3.5.8+incompatible",
    )
    go_repository(
        name = "com_github_uber_jaeger_client_go",
        importpath = "github.com/uber/jaeger-client-go",
        sum = "h1:D6wyKGCecFaSRUpo8lCVbaOOb6ThwMmTEbhRwtKR97o=",
        version = "v2.30.0+incompatible",
    )
    go_repository(
        name = "com_github_uber_jaeger_lib",
        importpath = "github.com/uber/jaeger-lib",
        sum = "h1:td4jdvLcExb4cBISKIpHuGoVXh+dVKhn2Um6rjCsSsg=",
        version = "v2.4.1+incompatible",
    )
    go_repository(
        name = "com_github_uber_ringpop_go",
        importpath = "github.com/uber/ringpop-go",
        sum = "h1:aBa/SHmmFRcAXA63k7uBheoTL8tCmH7L+OgktB1AF/o=",
        version = "v0.8.5",
    )
    go_repository(
        name = "com_github_uber_tchannel_go",
        importpath = "github.com/uber/tchannel-go",
        sum = "h1:Wi7SSfUQbJ4lPqS4trP1uAZvWu46oK0qz0tOodcfcbY=",
        version = "v1.34.4",
    )
    go_repository(
        name = "com_github_ugorji_go_codec",
        importpath = "github.com/ugorji/go/codec",
        sum = "h1:waO7eEiFDwidsBN6agj1vJQ4AG7lh2yqXyOXqhgQuyY=",
        version = "v1.3.1",
    )
    go_repository(
        name = "com_github_xanzy_ssh_agent",
        importpath = "github.com/xanzy/ssh-agent",
        sum = "h1:+/15pJfg/RsTxqYcX6fHqOXZwwMP+2VyYWJeWM2qQFM=",
        version = "v0.3.3",
    )
    go_repository(
        name = "com_github_xdg_go_pbkdf2",
        importpath = "github.com/xdg-go/pbkdf2",
        sum = "h1:Su7DPu48wXMwC3bs7MCNG+z4FhcyEuz5dlvchbq0B0c=",
        version = "v1.0.0",
    )
    go_repository(
        name = "com_github_xdg_go_scram",
        importpath = "github.com/xdg-go/scram",
        sum = "h1:bYKF2AEwG5rqd1BumT4gAnvwU/M9nBp2pTSxeZw7Wvs=",
        version = "v1.2.0",
    )
    go_repository(
        name = "com_github_xdg_go_stringprep",
        importpath = "github.com/xdg-go/stringprep",
        sum = "h1:XLI/Ng3O1Atzq0oBs3TWm+5ZVgkq2aqdlvP9JtoZ6c8=",
        version = "v1.0.4",
    )
    go_repository(
        name = "com_github_youmark_pkcs8",
        importpath = "github.com/youmark/pkcs8",
        sum = "h1:ilQV1hzziu+LLM3zUTJ0trRztfwgjqKnBWNtSRkbmwM=",
        version = "v0.0.0-20240726163527-a2c0da244d78",
    )
    go_repository(
        name = "com_github_yuin_goldmark",
        importpath = "github.com/yuin/goldmark",
        sum = "h1:fVcFKWvrslecOb/tg+Cc05dkeYx540o0FuFt3nUVDoE=",
        version = "v1.4.13",
    )
    go_repository(
        name = "com_google_cloud_go",
        importpath = "cloud.google.com/go",
        sum = "h1:eOI3/cP2VTU6uZLDYAoic+eyzzB9YyGmJ7eIjl8rOPg=",
        version = "v0.34.0",
    )
    go_repository(
        name = "com_google_cloud_go_compute_metadata",
        importpath = "cloud.google.com/go/compute/metadata",
        sum = "h1:pDUj4QMoPejqq20dK0Pg2N4yG9zIkYGdBtwLoEkH9Zs=",
        version = "v0.9.0",
    )
    go_repository(
        name = "dev_cel_expr",
        importpath = "cel.dev/expr",
        sum = "h1:1KrZg61W6TWSxuNZ37Xy49ps13NUovb66QLprthtwi4=",
        version = "v0.25.1",
    )
    go_repository(
        name = "in_gopkg_alecthomas_kingpin_v2",
        importpath = "gopkg.in/alecthomas/kingpin.v2",
        sum = "h1:jMFz6MfLP0/4fUyZle81rXUoxOBFi19VUFKVDOQfozc=",
        version = "v2.2.6",
    )
    go_repository(
        name = "in_gopkg_check_v1",
        importpath = "gopkg.in/check.v1",
        sum = "h1:Hei/4ADfdWqJk1ZMxUNpqntNwaWcugrBjAiHlqqRiVk=",
        version = "v1.0.0-20201130134442-10cb98267c6c",
    )
    go_repository(
        name = "in_gopkg_errgo_v2",
        importpath = "gopkg.in/errgo.v2",
        sum = "h1:0vLT13EuvQ0hNvakwLuFZ/jYrLp5F3kcWHXdRggjCE8=",
        version = "v2.1.0",
    )
    go_repository(
        name = "in_gopkg_warnings_v0",
        importpath = "gopkg.in/warnings.v0",
        sum = "h1:wFXVbFY8DY5/xOe1ECiWdKCzZlxgshcYVNkBHstARME=",
        version = "v0.1.2",
    )
    go_repository(
        name = "in_gopkg_yaml_v2",
        importpath = "gopkg.in/yaml.v2",
        sum = "h1:D8xgwECY7CYvx+Y2n4sBz93Jn9JRvxdiyyo8CTfuKaY=",
        version = "v2.4.0",
    )
    go_repository(
        name = "in_gopkg_yaml_v3",
        importpath = "gopkg.in/yaml.v3",
        sum = "h1:fxVm/GzAzEWqLHuvctI91KS9hhNmmWOoWu0XTYJS7CA=",
        version = "v3.0.1",
    )
    go_repository(
        name = "in_yaml_go_yaml_v3",
        importpath = "go.yaml.in/yaml/v3",
        sum = "h1:tfq32ie2Jv2UxXFdLJdh3jXuOzWiL1fo0bu/FbuKpbc=",
        version = "v3.0.4",
    )
    go_repository(
        name = "io_opentelemetry_go_auto_sdk",
        importpath = "go.opentelemetry.io/auto/sdk",
        sum = "h1:jXsnJ4Lmnqd11kwkBV2LgLoFMZKizbCi5fNZ/ipaZ64=",
        version = "v1.2.1",
    )
    go_repository(
        name = "io_opentelemetry_go_contrib_detectors_gcp",
        importpath = "go.opentelemetry.io/contrib/detectors/gcp",
        sum = "h1:kWRNZMsfBHZ+uHjiH4y7Etn2FK26LAGkNFw7RHv1DhE=",
        version = "v1.39.0",
    )
    go_repository(
        name = "io_opentelemetry_go_otel",
        importpath = "go.opentelemetry.io/otel",
        sum = "h1:8yPrr/S0ND9QEfTfdP9V+SiwT4E0G7Y5MO7p85nis48=",
        version = "v1.39.0",
    )
    go_repository(
        name = "io_opentelemetry_go_otel_metric",
        importpath = "go.opentelemetry.io/otel/metric",
        sum = "h1:d1UzonvEZriVfpNKEVmHXbdf909uGTOQjA0HF0Ls5Q0=",
        version = "v1.39.0",
    )
    go_repository(
        name = "io_opentelemetry_go_otel_sdk",
        importpath = "go.opentelemetry.io/otel/sdk",
        sum = "h1:nMLYcjVsvdui1B/4FRkwjzoRVsMK8uL/cj0OyhKzt18=",
        version = "v1.39.0",
    )
    go_repository(
        name = "io_opentelemetry_go_otel_sdk_metric",
        importpath = "go.opentelemetry.io/otel/sdk/metric",
        sum = "h1:cXMVVFVgsIf2YL6QkRF4Urbr/aMInf+2WKg+sEJTtB8=",
        version = "v1.39.0",
    )
    go_repository(
        name = "io_opentelemetry_go_otel_trace",
        importpath = "go.opentelemetry.io/otel/trace",
        sum = "h1:2d2vfpEDmCJ5zVYz7ijaJdOF59xLomrvj7bjt6/qCJI=",
        version = "v1.39.0",
    )
    go_repository(
        name = "io_rsc_pdf",
        importpath = "rsc.io/pdf",
        sum = "h1:k1MczvYDUvJBe93bYd7wrZLLUEcLZAuF824/I4e5Xr4=",
        version = "v0.1.1",
    )
    go_repository(
        name = "org_golang_google_appengine",
        importpath = "google.golang.org/appengine",
        sum = "h1:FZR1q0exgwxzPzp/aF+VccGrSfxfPpkBqjIIEq3ru6c=",
        version = "v1.6.7",
    )
    go_repository(
        name = "org_golang_google_genproto",
        importpath = "google.golang.org/genproto",
        sum = "h1:387Y+JbxF52bmesc8kq1NyYIp33dnxCw6eiA7JMsTmw=",
        version = "v0.0.0-20250115164207-1a7da9e5054f",
    )
    go_repository(
        name = "org_golang_google_genproto_googleapis_api",
        importpath = "google.golang.org/genproto/googleapis/api",
        sum = "h1:fCvbg86sFXwdrl5LgVcTEvNC+2txB5mgROGmRL5mrls=",
        version = "v0.0.0-20251202230838-ff82c1b0f217",
    )
    go_repository(
        name = "org_golang_google_genproto_googleapis_bytestream",
        importpath = "google.golang.org/genproto/googleapis/bytestream",
        sum = "h1:7FlucM2tFADtEDnIlDrR12KdRqV48B1GSTU1U6uKSiY=",
        version = "v0.0.0-20251124214823-79d6a2a48846",
    )
    go_repository(
        name = "org_golang_google_genproto_googleapis_rpc",
        importpath = "google.golang.org/genproto/googleapis/rpc",
        sum = "h1:gRkg/vSppuSQoDjxyiGfN4Upv/h/DQmIR10ZU8dh4Ww=",
        version = "v0.0.0-20251202230838-ff82c1b0f217",
    )
    go_repository(
        name = "org_golang_google_grpc",
        importpath = "google.golang.org/grpc",
        sum = "h1:sybAEdRIEtvcD68Gx7dmnwjZKlyfuc61Dyo9pGXXkKE=",
        version = "v1.79.3",
    )
    go_repository(
        name = "org_golang_google_grpc_cmd_protoc_gen_go_grpc",
        importpath = "google.golang.org/grpc/cmd/protoc-gen-go-grpc",
        sum = "h1:F29+wU6Ee6qgu9TddPgooOdaqsxTMunOoj8KA5yuS5A=",
        version = "v1.5.1",
    )
    go_repository(
        name = "org_golang_google_protobuf",
        importpath = "google.golang.org/protobuf",
        sum = "h1:AYd7cD/uASjIL6Q9LiTjz8JLcrh/88q5UObnmY3aOOE=",
        version = "v1.36.10",
    )
    go_repository(
        name = "org_golang_x_arch",
        importpath = "golang.org/x/arch",
        sum = "h1:c/Zle32i5ttqRXjdLyyHZESLD/bB90DCU1g9l/0YBDI=",
        version = "v0.22.0",
    )
    go_repository(
        name = "org_golang_x_crypto",
        importpath = "golang.org/x/crypto",
        sum = "h1:YLIA59K4fiNzHzjnZt2tUJQjQtUWfWbeHBqKtk3eScw=",
        version = "v0.54.0",
    )
    go_repository(
        name = "org_golang_x_exp",
        importpath = "golang.org/x/exp",
        sum = "h1:FRnLl4eNAQl8hwxVVC17teOw8kdjVDVAiFMtgUdTSRQ=",
        version = "v0.0.0-20231110203233-9a3e6036ecaa",
    )
    go_repository(
        name = "org_golang_x_exp_typeparams",
        importpath = "golang.org/x/exp/typeparams",
        sum = "h1:1P7xPZEwZMoBoz0Yze5Nx2/4pxj6nw9ZqHWXqP0iRgQ=",
        version = "v0.0.0-20231108232855-2478ac86f678",
    )
    go_repository(
        name = "org_golang_x_lint",
        importpath = "golang.org/x/lint",
        sum = "h1:VLliZ0d+/avPrXXH+OakdXhpJuEoBZuwh1m2j7U6Iug=",
        version = "v0.0.0-20210508222113-6edffad5e616",
    )
    go_repository(
        name = "org_golang_x_mod",
        importpath = "golang.org/x/mod",
        sum = "h1:MECBjubtXD7yj4HrhIUcywNaGeNVUdfVnxmPajOk4yk=",
        version = "v0.38.0",
    )
    go_repository(
        name = "org_golang_x_net",
        importpath = "golang.org/x/net",
        sum = "h1:K5+3DljvIuDG9/Jv9rvyMywYNFCQ9RSUY6OOTTkT+tE=",
        version = "v0.57.0",
    )
    go_repository(
        name = "org_golang_x_oauth2",
        importpath = "golang.org/x/oauth2",
        sum = "h1:hqK/t4AKgbqWkdkcAeI8XLmbK+4m4G5YeQRrmiotGlw=",
        version = "v0.34.0",
    )
    go_repository(
        name = "org_golang_x_sync",
        importpath = "golang.org/x/sync",
        sum = "h1:SZjpbeLmrCk4xhRSZFNZW5gFUeCeFgjekvI/+gfScek=",
        version = "v0.22.0",
    )
    go_repository(
        name = "org_golang_x_sys",
        importpath = "golang.org/x/sys",
        sum = "h1:o7XGOvZQCADBQQ4Y7VNq2dRWQR7JmOUW8Kxx4ZsNgWs=",
        version = "v0.47.0",
    )
    go_repository(
        name = "org_golang_x_telemetry",
        importpath = "golang.org/x/telemetry",
        sum = "h1:RJhm5l6Fo4rmEIcndxDllNhhf/fAx8qIm4t6A7vpm2A=",
        version = "v0.0.0-20260708182218-49f421fb7959",
    )
    go_repository(
        name = "org_golang_x_term",
        importpath = "golang.org/x/term",
        sum = "h1:NwWyBmoJCbfTHpxrWoZ9C6/VxOf7ic219I8xZZFdrf0=",
        version = "v0.45.0",
    )
    go_repository(
        name = "org_golang_x_text",
        importpath = "golang.org/x/text",
        sum = "h1:Ub2Z6/xjgF1WrYQz2nuITOEegKFtiIy+rieRJ5lHZKs=",
        version = "v0.40.0",
    )
    go_repository(
        name = "org_golang_x_time",
        importpath = "golang.org/x/time",
        sum = "h1:3X+R0qq1+64izd8es+EttB6qcY+JDlVmAhpRXl7gpzU=",
        version = "v0.0.0-20170927054726-6dc17368e09b",
    )
    go_repository(
        name = "org_golang_x_tools",
        importpath = "golang.org/x/tools",
        sum = "h1:3+hClM1aLL5mjMKm5ovokw9epgRXPuu2tILgismM6RE=",
        version = "v0.48.0",
    )
    go_repository(
        name = "org_golang_x_tools_go_expect",
        importpath = "golang.org/x/tools/go/expect",
        sum = "h1:jpBZDwmgPhXsKZC6WhL20P4b/wmnpsEAGHaNy0n/rJM=",
        version = "v0.1.1-deprecated",
    )
    go_repository(
        name = "org_golang_x_xerrors",
        importpath = "golang.org/x/xerrors",
        sum = "h1:go1bK/D/BFZV2I8cIQd1NKEZ+0owSTG1fDTci4IqFcE=",
        version = "v0.0.0-20200804184101-5ec99f83aff1",
    )
    go_repository(
        name = "org_gonum_v1_gonum",
        importpath = "gonum.org/v1/gonum",
        sum = "h1:5+ul4Swaf3ESvrOnidPp4GZbzf0mxVQpDCYUQE7OJfk=",
        version = "v0.16.0",
    )
    go_repository(
        name = "org_mongodb_go_mongo_driver_v2",
        importpath = "go.mongodb.org/mongo-driver/v2",
        sum = "h1:yXUhImUjjAInNcpTcAlPHiT7bIXhshCTL3jVBkF3xaE=",
        version = "v2.5.0",
    )
    go_repository(
        name = "org_uber_go_atomic",
        importpath = "go.uber.org/atomic",
        sum = "h1:ZvwS0R+56ePWxUNi+Atn9dWONBPp/AUETXlHW0DxSjE=",
        version = "v1.11.0",
    )
    go_repository(
        name = "org_uber_go_cadence",
        importpath = "go.uber.org/cadence",
        sum = "h1:nRjhQ1LMTxC7A2s21ex2hr/ECCbzoZKYNsVq/wJzEX8=",
        version = "v1.3.0",
    )
    go_repository(
        name = "org_uber_go_cff",
        importpath = "go.uber.org/cff",
        sum = "h1:UIO1A7Lx+tTjfNkWptztubwW/JhMLhVkGAbMiGy5gCM=",
        version = "v0.0.0-20240514225903-4cdfe9553c2c",
    )
    go_repository(
        name = "org_uber_go_dig",
        importpath = "go.uber.org/dig",
        sum = "h1:BACLhebsYdpQ7IROQ1AGPjrXcP5dF80U3gKoFzbaq/4=",
        version = "v1.19.0",
    )
    go_repository(
        name = "org_uber_go_fx",
        importpath = "go.uber.org/fx",
        sum = "h1:wE8mruvpg2kiiL1Vqd0CC+tr0/24XIB10Iwp2lLWzkg=",
        version = "v1.24.0",
    )
    go_repository(
        name = "org_uber_go_goleak",
        importpath = "go.uber.org/goleak",
        sum = "h1:2K3zAYmnTNqV73imy9J1T3WC+gmCePx2hEGkimedGto=",
        version = "v1.3.0",
    )
    go_repository(
        name = "org_uber_go_mock",
        importpath = "go.uber.org/mock",
        sum = "h1:hyF9dfmbgIX5EfOdasqLsWD6xqpNZlXblLB/Dbnwv3Y=",
        version = "v0.6.0",
    )
    go_repository(
        name = "org_uber_go_multierr",
        importpath = "go.uber.org/multierr",
        sum = "h1:blXXJkSxSSfBVBlC76pxqeO+LN3aDfLQo+309xJstO0=",
        version = "v1.11.0",
    )
    go_repository(
        name = "org_uber_go_net_metrics",
        importpath = "go.uber.org/net/metrics",
        sum = "h1:c2i1L/L3/XCgODG1vuYuA6K4v4VI18EOdDf1aF1YNg4=",
        version = "v1.4.0",
    )
    go_repository(
        name = "org_uber_go_thriftrw",
        importpath = "go.uber.org/thriftrw",
        sum = "h1:F6aIyIg6B1+fx7aiylVyqDdTG8mtLQfREfOdlm6u+mI=",
        version = "v1.34.0",
    )
    go_repository(
        name = "org_uber_go_tools",
        importpath = "go.uber.org/tools",
        sum = "h1:0mgffUl7nfd+FpvXMVz4IDEaUSmT1ysygQC7qYo7sG4=",
        version = "v0.0.0-20190618225709-2cfd321de3ee",
    )
    go_repository(
        name = "org_uber_go_yarpc",
        importpath = "go.uber.org/yarpc",
        sum = "h1:qWf6ptFP3COM4TDWcXxBpzhrfaT3gt18cU+adSPKTlg=",
        version = "v1.89.0",
    )
    go_repository(
        name = "org_uber_go_yarpc_internal_examples",
        importpath = "go.uber.org/yarpc/internal/examples",
        sum = "h1:jPEEACCKiDluWTvbEChbCsxcNC5wKQaR3Ks1j3JlspU=",
        version = "v0.0.0-20230831212929-ccef8c01afa8",
    )
    go_repository(
        name = "org_uber_go_zap",
        importpath = "go.uber.org/zap",
        sum = "h1:IZzaP1Fv73/T/pBMLk4VutPl36uNC+OSUh3JLG3FIjo=",
        version = "v1.28.0",
    )

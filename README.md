# full-url-rewrite-traefik-plugin

Allows to use regex to match and rewrite full request URL. It's useful when rewrite rule needs to take into
account parts of request host name which is unavailable with built-in `ReplacePathRegex`, e.g. when there's
a requirement to change `https://company1.example.com` to `https://saas.com/company1`. This allows to change the entire URL including hostname and path without the need to perform unnecessary [client redirect](https://doc.traefik.io/traefik/middlewares/http/redirectregex/) which saves a round-trip and doesn't expose the backend routing details.

## Configuration

### Static

```yaml
experimental:
  plugins:
    fullUrlRewrite:
      moduleName: github.com/e-flux-platform/full-url-rewrite-traefik-plugin
      version: v0.0.6
```

### Dynamic

In order to configure URL rewriting you should create a [middleware](https://docs.traefik.io/middlewares/overview/) in your dynamic configuration and define the rewriting rule.

The following example creates and uses the full URL rewrite middleware plugin to change target URL host and add a path prefix that is taken from the original URL subdomain:

```yaml
http:
  routes:
    myRouter:
      rule: Host(`localhost`)
      service: my-service
      middlewares:
        - myRewriteMiddleware

  services:
    myService:
      loadBalancer:
        servers:
          - url: http://127.0.0.1

middlewares:
  myRewriteMiddleware:
    plugin:
      fullUrlRewrite:
        regex: ^//(\w+)\.example\.com/(.+)
        replacement: //saas.com/$1/$2
```

Note that both `regex` and `replacement` values begin with `//`. Request URLs in the context of a reverse proxy are non-absolute, there's simply no notion of URL scheme and it couldn't be matched and/or changed.

By default, rewritten value of host is passed downstream in the `Host` HTTP header. This can be turned off using [passHostHeader setting](https://doc.traefik.io/traefik/routing/services/#pass-host-header).

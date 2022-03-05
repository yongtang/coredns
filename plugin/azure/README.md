# azure

## Name

*azure* - enables serving zone data from Microsoft Azure DNS service.

## Description

The azure plugin is useful for serving zones from Microsoft Azure DNS. The *azure* plugin supports
all the DNS records supported by Azure, viz. A, AAAA, CNAME, MX, NS, PTR, SOA, SRV, and TXT
record types. NS record type is not supported by azure private DNS.

## Syntax

~~~ txt
azure RESOURCE_GROUP:ZONE... {
    tenant AZURE_TENANT_ID
    client AZURE_CLIENT_ID
    secret AZURE_CLIENT_SECRET
    subscription AZURE_SUBSCRIPTION_ID
    environment AZURE_ENVIRONMENT
    fallthrough [ZONES...]
    access private
}
~~~

*   **RESOURCE_GROUP:ZONE** is the resource group to which the hosted zones belongs on Azure,
    and **ZONE** the zone that contains data.

*   **AZURE_CLIENT_ID** and **AZURE_CLIENT_SECRET** are the credentials for Azure, and `tenant` specifies the
    **AZURE_TENANT_ID** to be used. **AZURE_SUBSCRIPTION_ID** is the subscription ID. All of these are needed
    to access the data in Azure. If `tenant`, `client`, `secret`, or `subscription` are not specified
    in Corefile, then values will be obtained through environmental variables `AZURE_TENENT_ID`,
    `AZURE_CLIENT_ID`, `AZURE_CLIENT_SECRET`, and `AZURE_SUBSCRIPTION_ID` respectively.
    Note `secret` field in Corefile has been deprecated and may be removed in future releases. Environmental
    variable **AZURE_CLIENT_SECRET** should be used instead.

*   **AZURE_ENVIRONMENT** specifies the Azure **Environment**. This value is optional, and can be specified
    through `environment` field in Corefile, or through environmental variable `AZURE_ENVIRONMENT`.

*   `fallthrough` If zone matches and no record can be generated, pass request to the next plugin.
    If **ZONES** is omitted, then fallthrough happens for all zones for which the plugin is
    authoritative.

*   `access`  specifies if the zone is `public` or `private`. Default is `public`.

## Authentication

Azure plugin uses [environment-based authentication](https://docs.microsoft.com/en-us/azure/developer/go/azure-sdk-authorization#use-environment-based-authentication),
where there is a list of accepted environmental variables. Note in additional to passing those environmental variables,
it is also possible to use `tenant`, `client`, `subscription`, and `environment` to override corresponding environmental variables.
Note `secret` field in Corefile has been deprecated and may be removed in future releases. Users should pass secret through
environmental variable **AZURE_CLIENT_SECRET** instead.

## Examples

Enable the *azure* plugin with Azure credentials for private zones `example.org`, `example.private`:

~~~ txt
example.org {
    azure resource_group_foo:example.org resource_group_foo:example.private {
      tenant 123abc-123abc-123abc-123abc
      client 123abc-123abc-123abc-234xyz
      subscription 123abc-123abc-123abc-563abc
      secret mysecret # Deprecated, uses environmental variable `AZURE_CLIENT_SECRET` instead.
      access private
    }
}
~~~

## See Also

The [Azure DNS Overview](https://docs.microsoft.com/en-us/azure/dns/dns-overview).

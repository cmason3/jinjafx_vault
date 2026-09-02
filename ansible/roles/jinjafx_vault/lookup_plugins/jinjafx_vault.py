import requests, json
from ansible.plugins.lookup import LookupBase
from ansible.errors import AnsibleError

class LookupModule(LookupBase):
  def run(self, terms, variables=None, **kwargs):
    if len(terms) == 2:
      headers = { 'X-Vault-Token': variables['jinjafx_vault_login']['json']['token'] }
      verify = variables.get('jinjafx_vault_verify', True)
      timeout = variables.get('jinjafx_vault_timeout', 5)
      namespace = terms[0]
      variable = terms[1]

      if not verify:
        requests.packages.urllib3.disable_warnings(requests.packages.urllib3.exceptions.InsecureRequestWarning)

      if (r := requests.get(variables['jinjafx_vault_url'] + f'/v1/data/{namespace}/{variable}', headers=headers, verify=verify, timeout=timeout)).status_code == 200:
        return [json.loads(r.text)['data']]

      else:
        raise AnsibleError(f'Unable to lookup JinjaFx Vault variable "{variable}" within namespace "{namespace}"')

    raise AnsibleError('Invalid arguments provided to JinjaFx Vault lookup')

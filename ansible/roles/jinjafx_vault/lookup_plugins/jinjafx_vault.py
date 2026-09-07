import requests, json
from ansible.plugins.lookup import LookupBase
from ansible.errors import AnsibleError

class LookupModule(LookupBase):
  def run(self, terms, variables=None, **kwargs):
    headers = { 'X-Vault-Token': variables['jinjafx_vault_login']['json']['token'] }
    verify = variables.get('jinjafx_vault_verify', True)
    timeout = variables.get('jinjafx_vault_timeout', 5)

    if not verify:
      requests.packages.urllib3.disable_warnings(requests.packages.urllib3.exceptions.InsecureRequestWarning)

    if len(terms) == 2:
      namespace = terms[0]
      variable = terms[1]

      if (r := requests.get(variables['jinjafx_vault_url'] + f'/v1/data/{namespace}/{variable}', headers=headers, verify=verify, timeout=timeout)).status_code == 200:
        return [json.loads(r.text)['data']]

      else:
        raise AnsibleError(f'JinjaFx Vault - Unable to get variable "{variable}" within namespace "{namespace}"')

    elif len(terms) == 1:
      namespace = terms[0]

      if (r := requests.get(variables['jinjafx_vault_url'] + f'/v1/data/{namespace}', headers=headers, verify=verify, timeout=timeout)).status_code == 200:
        obj = json.loads(r.text)
        result = {}

        for k, v in obj.items():
          result[k] = v['data']

        return [result]

      else:
        raise AnsibleError(f'JinjaFx Vault - Unable to get namespace "{namespace}"')

    raise AnsibleError('JinjaFx Vault - Invalid arguments provided to lookup function')

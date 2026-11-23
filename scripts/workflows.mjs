import { readdir, readFile } from 'node:fs/promises';
import { pathToFileURL } from 'node:url';
import { parse } from 'yaml';

export function workflowErrors(workflow, filename = '') {
  const errors = [];
  const checkUse = use => {
    if (use && !use.startsWith('./') && !/^[\w./-]+@[a-f0-9]{40}$/.test(use)) errors.push(`unresolved Action pin: ${use}`);
  };
  if (filename === 'publish.yml' && !workflow.jobs?.binaries?.environment) errors.push('binary publication missing approval environment');
  if (filename === 'image.yml' && !workflow.jobs?.publish?.environment) errors.push('image publication missing approval environment');
  if (filename === 'pages.yml') {
    const producer = workflow.on?.workflow_run;
    if (!producer?.workflows?.includes('CI') || !producer.types?.includes('completed')) errors.push('documentation publication must follow completed CI');
    const source = workflow.jobs?.source;
    const sourceCondition = source?.if ?? '';
    if (!sourceCondition.includes("vars.PUBLISH_DOCS == 'true'") ||
        !sourceCondition.includes("github.event.workflow_run.conclusion == 'success'") ||
        !sourceCondition.includes("github.event.workflow_run.event == 'push'") ||
        !sourceCondition.includes('github.event.workflow_run.head_repository.full_name == github.repository')) {
      errors.push('automatic documentation publication must be opt-in, successful, push-only and same-repository');
    }
    const sourceScript = source?.steps?.find(step => step.id === 'source')?.run ?? '';
    if (!sourceScript.includes('classify(paths)') || !sourceScript.includes('changes.docs') || !sourceScript.includes('sha!==current')) {
      errors.push('automatic documentation publication must require documentation changes at the current revision');
    }
    if (workflow.jobs?.build?.needs !== 'source') errors.push('documentation build must depend on source validation');
    if (workflow.jobs?.deploy?.needs !== 'build') errors.push('documentation deployment must depend on successful build');
  }
  if (workflow.on && Object.hasOwn(workflow.on, 'pull_request_target')) errors.push('privileged PR event is prohibited');
  for (const [permission, value] of Object.entries(workflow.permissions ?? {})) {
    if (value === 'write') errors.push(`workflow-wide write permission: ${permission}`);
  }
  for (const [name, job] of Object.entries(workflow.jobs ?? {})) {
    checkUse(job.uses);
    if (job.permissions?.['id-token'] === 'write' && name !== 'attest' && !job.uses && !(filename === 'pages.yml' && name === 'deploy')) errors.push(`OIDC outside attestation job: ${name}`);
    if (['ci.yml', 'validate.yml'].includes(filename) && Object.values(job.permissions ?? {}).includes('write')) errors.push(`validation job has write permission: ${name}`);
    for (const step of job.steps ?? []) {
      checkUse(step.uses);
      if ((step.with?.['predicate-type'] || step.with?.predicate || step.with?.['predicate-path']) && !step.uses?.startsWith('actions/attest@')) errors.push(`custom predicate requires generic actions/attest: ${name}`);
      if (step.uses?.startsWith('actions/checkout@') && step.with?.['persist-credentials'] !== false) errors.push(`checkout persists credentials: ${name}`);
      if (/\$\{\{[^}]*(github\.event\.|inputs\.)/.test(step.run ?? '')) errors.push(`untrusted expression interpolated into shell: ${name}`);
      if (/npm ci\b/.test(step.run ?? '') && !step.run.includes('--ignore-scripts')) errors.push(`npm lifecycle scripts enabled: ${name}`);
    }
  }
  return errors;
}

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  const directory = new URL('../.github/workflows/', import.meta.url);
  for (const filename of await readdir(directory)) {
    if (!/\.ya?ml$/.test(filename)) continue;
    const workflow = parse(await readFile(new URL(filename, directory), 'utf8'));
    const errors = workflowErrors(workflow, filename);
    if (errors.length) throw new Error(`${filename}: ${errors.join('; ')}`);
  }
  console.log('Workflow policy: direct pins, token scopes, checkout, shell input and install boundaries checked');
}
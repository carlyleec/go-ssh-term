import { spawn } from 'node:child_process'
import { createHash } from 'node:crypto'
import {
  mkdir,
  readdir,
  readFile,
  rename,
  rm,
  writeFile,
} from 'node:fs/promises'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import openapiTS, { astToString } from 'openapi-typescript'
import ts from 'typescript'
import { generateZod } from './gen-api-zod.mjs'

const root = fileURLToPath(new URL('../../', import.meta.url))
const outputs = [
  'openapi/api.json',
  'frontend/src/api/generated/schema.gen.ts',
  'frontend/src/api/generated/zod.gen.ts',
]
function exportSpec() {
  return new Promise((accept, reject) => {
    const env = { ...process.env }
    delete env.GOROOT
    const child = spawn('go', ['run', './cmd/openapi'], {
      cwd: root,
      env,
      stdio: ['ignore', 'pipe', 'inherit'],
    })
    const chunks = []
    child.stdout.on('data', (chunk) => chunks.push(chunk))
    child.on('error', reject)
    child.on('close', (code) =>
      code === 0
        ? accept(Buffer.concat(chunks).toString())
        : reject(new Error(`OpenAPI export exited ${code}`)),
    )
  })
}

export async function generate(frontendOnly = false) {
  const json = frontendOnly
    ? await readFile(resolve(root, outputs[0]), 'utf8')
    : await exportSpec()
  const spec = JSON.parse(json)
  // Finish both generators before touching any last-good artifact.
  const zod = generateZod(spec)
  const types =
    '// Generated from OpenAPI. DO NOT EDIT.\n' +
    astToString(
      await openapiTS(spec, {
        rootTypes: true,
        transform(schema) {
          if (schema.format === 'binary')
            return ts.factory.createTypeReferenceNode('File')
        },
      }),
    )
  const files = (frontendOnly ? outputs.slice(1) : outputs).map(
    (path, index) => ({
      path: resolve(root, path),
      data: (frontendOnly ? [types, zod] : [json, types, zod])[index],
    }),
  )
  const staged = []
  try {
    for (const file of files) {
      if ((await readFile(file.path, 'utf8').catch(() => null)) === file.data)
        continue
      await mkdir(dirname(file.path), { recursive: true })
      const temp = `${file.path}.${process.pid}.tmp`
      staged.push({ ...file, temp })
      await writeFile(temp, file.data)
    }
    for (const file of staged) await rename(file.temp, file.path)
  } finally {
    await Promise.all(staged.map((file) => rm(file.temp, { force: true })))
  }
  console.log('[contract] Generated OpenAPI types and Zod schemas')
}

export async function fingerprint() {
  const hash = createHash('sha256')
  async function visit(path) {
    for (const entry of (
      await readdir(resolve(root, path), { withFileTypes: true })
    ).sort((a, b) => a.name.localeCompare(b.name))) {
      const file = `${path}/${entry.name}`
      if (entry.isDirectory() && entry.name !== 'node_modules')
        await visit(file)
      else if (/\.(go|sql|mjs)$/.test(entry.name))
        hash.update(file).update(await readFile(resolve(root, file)))
    }
  }
  for (const path of ['cmd', 'internal', 'db', 'frontend/scripts'])
    await visit(path)
  for (const path of [
    'go.mod',
    'go.sum',
    'frontend/scripts/package.json',
    'frontend/scripts/bun.lock',
  ])
    hash.update(await readFile(resolve(root, path)))
  return hash.digest('hex')
}
async function watch() {
  let last
  let pending
  // Polling also observes bind-mounted edits in Docker. Awaiting each run keeps
  // generation serialized; a subsequent edit is picked up by the next poll.
  while (true) {
    let current
    try {
      current = await fingerprint()
    } catch (error) {
      console.error('[contract] Could not read inputs', error)
      await new Promise((resolve) => setTimeout(resolve, 500))
      continue
    }
    if (current !== last && current === pending) {
      try {
        await generate()
      } catch (error) {
        console.error('[contract]', error)
      }
      last = current
    }
    pending = current
    await new Promise((resolve) => setTimeout(resolve, 500))
  }
}
if (
  process.argv[1] &&
  resolve(process.argv[1]) === fileURLToPath(import.meta.url)
) {
  const mode = process.argv[2]
  try {
    if (mode === 'watch') await watch()
    else if (mode === 'api' || mode === 'generate')
      await generate(mode === 'api')
    else throw new Error('Usage: contract.mjs generate|api|watch')
  } catch (error) {
    console.error(error)
    process.exitCode = 1
  }
}

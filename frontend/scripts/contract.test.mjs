import { expect, test } from 'bun:test'
import { readFile } from 'node:fs/promises'
import { contracts } from '../src/api/generated/zod.gen.ts'
import { generateZod } from './gen-api-zod.mjs'

const spec = JSON.parse(
  await readFile(new URL('../../openapi/api.json', import.meta.url)),
)
test('deterministic Zod output matches saved artifact', async () => {
  expect(generateZod(spec)).toBe(
    await readFile(
      new URL('../src/api/generated/zod.gen.ts', import.meta.url),
      'utf8',
    ),
  )
})
test('unsupported constraints and references fail closed', () => {
  for (const change of [
    { minLength: -1 },
    { pattern: 'x' },
    { format: 'uuid' },
    { anyOf: [] },
    { $ref: 'https://example.test/schema' },
  ]) {
    const changed = structuredClone(spec)
    Object.assign(changed.components.schemas.Account.properties.id, change)
    expect(() => generateZod(changed)).toThrow()
  }
})
test('connection wire constraints preserve Unicode lengths and numeric bounds', () => {
  const schema = contracts.createConnection.request['application/json']
  const value = {
    name: '😀'.repeat(256),
    host: 'bastion',
    port: 22,
    username: 'demo',
    ssh_key_id: 'key',
  }
  expect(schema.safeParse(value).success).toBe(true)
  for (const change of [
    { name: '' },
    { name: '😀'.repeat(257) },
    { port: 0 },
    { port: 65536 },
    { port: 22.5 },
    { account_id: 'other' },
  ]) {
    expect(schema.safeParse({ ...value, ...change }).success).toBe(false)
  }
  expect(
    contracts.listConnections.responses['200']['application/json'].parse({
      connections: [],
    }),
  ).toEqual({ connections: [] })
  expect(
    contracts.deleteKey.responses['409']['application/json'].parse({
      error: 'referenced',
    }),
  ).toEqual({ error: 'referenced' })
})
test('schemas validate envelopes, errors, files and empty responses', () => {
  const account = contracts.currentUser.responses['200']['application/json']
  expect(
    account.safeParse({ account: { id: 'id', display_name: 'Alice' } }).success,
  ).toBe(true)
  expect(account.safeParse({ id: 'id', display_name: 'Alice' }).success).toBe(
    false,
  )
  expect(
    account.safeParse({ account: { id: 12, display_name: 'Alice' } }).success,
  ).toBe(false)
  expect(
    contracts.listKeys.responses['200']['application/json'].parse({ keys: [] }),
  ).toEqual({ keys: [] })
  expect(contracts.deleteKey.responses['204'].safeParse({}).success).toBe(false)
  expect(
    contracts.deleteKey.responses['204'].safeParse(undefined).success,
  ).toBe(true)
  expect(
    contracts.uploadKey.request['multipart/form-data'].safeParse({
      name: 'Demo',
      private_key: new File(['key'], 'key'),
    }).success,
  ).toBe(true)
  expect(
    contracts.uploadKey.request['multipart/form-data'].safeParse({
      name: 'Demo',
      private_key: 'key',
    }).success,
  ).toBe(false)
  expect(
    contracts.uploadKey.responses['413']['application/json'].parse({
      error: 'too big',
    }),
  ).toEqual({ error: 'too big' })
})

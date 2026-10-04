const annotations = new Set([
  'description',
  'title',
  'examples',
  'example',
  'deprecated',
  'readOnly',
  'writeOnly',
])
const supported = new Set([
  '$ref',
  'type',
  'properties',
  'required',
  'additionalProperties',
  'items',
  'format',
  'contentMediaType',
  'minLength',
  'maxLength',
  'minimum',
  'maximum',
  ...annotations,
])
const quote = JSON.stringify

export function generateZod(spec) {
  if (spec.openapi !== '3.1.0') throw new Error('Expected OpenAPI 3.1.0')
  const definitions = spec.components?.schemas ?? {}
  const emitted = new Map()
  const active = new Set()
  const fail = (where, reason) => {
    throw new Error(`${where}: ${reason}`)
  }
  function reference(ref) {
    const prefix = '#/components/schemas/'
    if (!ref.startsWith(prefix)) fail(ref, 'unsupported reference')
    const name = ref.slice(prefix.length)
    if (!/^[A-Za-z_$][\w$]*$/.test(name) || !Object.hasOwn(definitions, name))
      fail(ref, 'invalid schema reference')
    if (active.has(name)) fail(ref, 'recursive schemas are not supported')
    if (!emitted.has(name)) {
      active.add(name)
      const code = schema(definitions[name], name)
      active.delete(name)
      emitted.set(name, `export const ${name}Schema = ${code}`)
    }
    return `${name}Schema`
  }
  function schema(value, where) {
    if (value === true) return 'z.unknown()'
    if (value === false) return 'z.never()'
    if (!value || typeof value !== 'object' || Array.isArray(value))
      fail(where, 'invalid schema')
    for (const key of Object.keys(value))
      if (!supported.has(key)) fail(where, `unsupported keyword ${key}`)
    if (value.$ref) {
      if (
        Object.keys(value).some(
          (key) => key !== '$ref' && !annotations.has(key),
        )
      )
        fail(where, 'reference siblings are not supported')
      return reference(value.$ref)
    }
    if (Array.isArray(value.type)) {
      if (value.type.length !== 2 || !value.type.includes('null'))
        fail(where, 'unsupported type union')
      return `${schema({ ...value, type: value.type.find((type) => type !== 'null') }, where)}.nullable()`
    }
    const allowedByType = {
      object: ['properties', 'required', 'additionalProperties'],
      array: ['items'],
      string: ['format', 'contentMediaType', 'minLength', 'maxLength'],
      integer: ['format', 'minimum', 'maximum'],
      number: ['minimum', 'maximum'],
      boolean: [],
      null: [],
    }
    for (const key of Object.keys(value)) {
      if (
        key !== 'type' &&
        !annotations.has(key) &&
        !allowedByType[value.type]?.includes(key)
      )
        fail(where, `unsupported ${key} for ${value.type}`)
    }
    switch (value.type) {
      case undefined:
        if (Object.keys(value).every((key) => annotations.has(key)))
          return 'z.unknown()'
        return fail(where, 'missing type')
      case 'object': {
        const properties = value.properties ?? {}
        const required = value.required ?? []
        if (
          !Array.isArray(required) ||
          required.some((key) => !Object.hasOwn(properties, key))
        )
          fail(where, 'invalid required properties')
        const fields = Object.entries(properties)
          .sort(([a], [b]) => a.localeCompare(b))
          .map(
            ([key, child]) =>
              `${quote(key)}: ${schema(child, `${where}.${key}`)}${required.includes(key) ? '' : '.optional()'}`,
          )
        const object = `{${fields.join(', ')}}`
        if (value.additionalProperties === false)
          return `z.strictObject(${object})`
        if (
          value.additionalProperties === undefined ||
          value.additionalProperties === true
        )
          return `z.looseObject(${object})`
        return `z.object(${object}).catchall(${schema(value.additionalProperties, `${where}.*`)})`
      }
      case 'array':
        if (!Object.hasOwn(value, 'items')) fail(where, 'array items missing')
        return `z.array(${schema(value.items, `${where}[]`)})`
      case 'string': {
        if (
          value.contentMediaType &&
          !(
            value.format === 'binary' &&
            value.contentMediaType === 'application/octet-stream'
          )
        )
          fail(where, 'unsupported contentMediaType')
        if (value.format === 'binary') {
          if (value.minLength !== undefined || value.maxLength !== undefined)
            fail(where, 'binary string length constraints are unsupported')
          return 'z.file()'
        }
        if (value.format && value.format !== 'date-time')
          fail(where, `unsupported format ${value.format}`)
        let code =
          value.format === 'date-time'
            ? 'z.iso.datetime({ offset: true })'
            : 'z.string()'
        for (const [keyword, operator] of [
          ['minLength', '>='],
          ['maxLength', '<='],
        ]) {
          if (value[keyword] === undefined) continue
          if (!Number.isSafeInteger(value[keyword]) || value[keyword] < 0)
            fail(where, `invalid ${keyword}`)
          // JSON Schema counts Unicode code points, not UTF-16 code units.
          code += `.refine((value) => [...value].length ${operator} ${value[keyword]}, ${quote(keyword)})`
        }
        return code
      }
      case 'integer':
      case 'number': {
        if (value.format && value.format !== 'int64')
          fail(where, `unsupported format ${value.format}`)
        let code = 'z.number()'
        for (const [keyword, method] of [
          ['minimum', 'min'],
          ['maximum', 'max'],
        ]) {
          if (value[keyword] === undefined) continue
          if (
            typeof value[keyword] !== 'number' ||
            !Number.isFinite(value[keyword])
          )
            fail(where, `invalid ${keyword}`)
          code += `.${method}(${value[keyword]})`
        }
        if (value.type === 'integer')
          code += '.refine(Number.isInteger, "Expected integer")'
        return code
      }
      case 'boolean':
        return 'z.boolean()'
      case 'null':
        return 'z.null()'
      default:
        return fail(where, `unsupported type ${value.type}`)
    }
  }
  const operations = []
  const ids = new Set()
  for (const [path, item] of Object.entries(spec.paths).sort()) {
    for (const method of [
      'get',
      'post',
      'put',
      'patch',
      'delete',
      'head',
      'options',
    ]) {
      const op = item[method]
      if (!op) continue
      const id = op.operationId
      if (!id || ids.has(id)) fail(path, 'missing or duplicate operationId')
      ids.add(id)
      function content(media, where) {
        if (!media || Object.keys(media).length === 0) return 'z.undefined()'
        const entries = Object.entries(media).map(([type, value]) => {
          if (
            !['application/json', 'multipart/form-data'].includes(type) ||
            !value.schema ||
            value.encoding
          )
            fail(where, `unsupported media ${type}`)
          return `${quote(type)}: ${schema(value.schema, where)}`
        })
        return `{${entries.join(', ')}}`
      }
      if (op.requestBody?.$ref)
        fail(id, 'request-body references are unsupported')
      const request = op.requestBody
        ? content(op.requestBody.content, `${id} request`)
        : 'z.undefined()'
      const responses = Object.entries(op.responses)
        .sort()
        .map(([status, response]) => {
          if (response.$ref) fail(id, 'response references are unsupported')
          return `${quote(status)}: ${content(response.content, `${id} ${status}`)}`
        })
      operations.push(
        `${quote(id)}: { request: ${request}, responses: {${responses.join(', ')}} }`,
      )
    }
  }
  return (
    '// Generated from OpenAPI. DO NOT EDIT.\nimport { z } from "zod"\n\n' +
    [...emitted.values()].join('\n') +
    '\n\nexport const contracts = {\n' +
    operations.join(',\n') +
    '\n} as const\n'
  )
}

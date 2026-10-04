import { expect, test } from 'bun:test'
import { keyUploadSchema } from '~/routes/_authed/keys/-components/key-drawer'
import { registrationSchema } from '~/routes/login/index'

const validFile = new File(['key'], 'key')

test('display-name schema counts Unicode code points after trimming without transforming form values', () => {
  const value = `  ${'😀'.repeat(64)}  `
  expect(registrationSchema.parse({ displayName: value }).displayName).toBe(
    value,
  )
  for (const invalid of ['', '   ', '😀'.repeat(65), 'Ca\u0000m']) {
    expect(
      registrationSchema
        .safeParse({ displayName: invalid })
        .error?.issues.map((issue) => issue.message),
    ).toEqual(['Enter 1–64 characters without control characters.'])
  }
})

test('key names preserve validation priority and reject excessive raw UTF-8 bytes', () => {
  const cases = [
    [' '.repeat(257), 'Enter a name for this key.'],
    ['😀'.repeat(65), 'Use at most 64 characters.'],
    ['Lab\u0000', 'The name cannot contain control characters.'],
    [` ${'😀'.repeat(64)}`, 'The name is too long. Remove extra whitespace.'],
    [`Lab${' '.repeat(254)}`, 'The name is too long. Remove extra whitespace.'],
  ]
  for (const [value, message] of cases) {
    expect(
      keyUploadSchema
        .safeParse({ name: value, file: validFile })
        .error?.issues.map((issue) => issue.message),
    ).toEqual([message])
  }
})

test('key names accept the character and byte boundaries without trimming form state', () => {
  for (const value of ['😀'.repeat(64), `Lab${' '.repeat(253)}`, '  Lab  ']) {
    expect(keyUploadSchema.parse({ name: value, file: validFile }).name).toBe(
      value,
    )
  }
})

test('private-key files must be present, nonempty, and at most 16 KiB', () => {
  for (const value of [null, new File([], 'empty')]) {
    expect(
      keyUploadSchema.safeParse({ name: 'Lab', file: value }).error?.issues[0]
        ?.message,
    ).toBe('Choose a private-key file.')
  }
  for (const size of [1, 16_384]) {
    const file = new File(['x'.repeat(size)], 'key')
    expect(keyUploadSchema.parse({ name: 'Lab', file }).file).toBe(file)
  }
  expect(
    keyUploadSchema.safeParse({
      name: 'Lab',
      file: new File(['x'.repeat(16_385)], 'large'),
    }).error?.issues[0]?.message,
  ).toBe('The private-key file must be at most 16 KiB.')
})

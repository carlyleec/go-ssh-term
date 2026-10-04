// Generated from OpenAPI. DO NOT EDIT.
import { z } from "zod"

export const LoginBeginInputBodySchema = z.strictObject({})
export const CredentialDescriptorSchema = z.strictObject({"id": z.string(), "transports": z.array(z.string()).nullable().optional(), "type": z.string()})
export const HMACGetSecretInputsSchema = z.strictObject({"salt1": z.string(), "salt2": z.string().optional()})
export const LargeBlobInputsSchema = z.strictObject({"read": z.boolean().optional(), "support": z.string().optional(), "write": z.string().optional()})
export const PRFValuesSchema = z.strictObject({"first": z.string(), "second": z.string().optional()})
export const PRFInputsSchema = z.strictObject({"eval": PRFValuesSchema.optional(), "evalByCredential": z.object({}).catchall(PRFValuesSchema).optional()})
export const AuthenticationExtensionsSchema = z.strictObject({"appid": z.string().optional(), "appidExclude": z.string().optional(), "credBlob": z.string().optional(), "credentialProtectionPolicy": z.string().optional(), "credProps": z.boolean().optional(), "enforceCredentialProtectionPolicy": z.boolean().optional(), "getCredBlob": z.boolean().optional(), "hmacCreateSecret": z.boolean().optional(), "hmacGetSecret": HMACGetSecretInputsSchema.optional(), "largeBlob": LargeBlobInputsSchema.optional(), "minPinLength": z.boolean().optional(), "prf": PRFInputsSchema.optional(), "remoteClientDataJSON": z.string().optional(), "uvm": z.boolean().optional()})
export const PublicKeyCredentialRequestOptionsSchema = z.strictObject({"allowCredentials": z.array(CredentialDescriptorSchema).nullable().optional(), "challenge": z.string(), "extensions": AuthenticationExtensionsSchema.optional(), "hints": z.array(z.string()).nullable().optional(), "rpId": z.string().optional(), "timeout": z.number().refine(Number.isInteger, "Expected integer").optional(), "userVerification": z.string().optional()})
export const CredentialAssertionSchema = z.strictObject({"mediation": z.string().optional(), "publicKey": PublicKeyCredentialRequestOptionsSchema})
export const AuthErrorBodySchema = z.strictObject({"error": z.string()})
export const CredentialPropertiesOutputSchema = z.strictObject({"rk": z.boolean().optional()})
export const HMACGetSecretOutputsSchema = z.strictObject({"output1": z.string().optional(), "output2": z.string().optional()})
export const LargeBlobOutputsSchema = z.strictObject({"blob": z.string().optional(), "supported": z.boolean().optional(), "written": z.boolean().optional()})
export const PRFOutputsSchema = z.strictObject({"enabled": z.boolean().optional(), "results": PRFValuesSchema.optional()})
export const AuthenticationExtensionsClientOutputsSchema = z.strictObject({"appid": z.boolean().optional(), "appidExclude": z.boolean().optional(), "credProps": CredentialPropertiesOutputSchema.optional(), "hmacCreateSecret": z.boolean().optional(), "hmacGetSecret": HMACGetSecretOutputsSchema.optional(), "largeBlob": LargeBlobOutputsSchema.optional(), "prf": PRFOutputsSchema.optional(), "remoteClientDataJSON": z.boolean().optional()})
export const AuthenticatorAssertionResponseSchema = z.strictObject({"authenticatorData": z.string(), "clientDataJSON": z.string(), "signature": z.string(), "userHandle": z.string().optional()})
export const CredentialAssertionResponseSchema = z.strictObject({"authenticatorAttachment": z.string().optional(), "clientExtensionResults": AuthenticationExtensionsClientOutputsSchema.optional(), "id": z.string(), "rawId": z.string(), "response": AuthenticatorAssertionResponseSchema, "type": z.string()})
export const AccountSchema = z.strictObject({"display_name": z.string(), "id": z.string()})
export const AccountBodySchema = z.strictObject({"account": AccountSchema})
export const CurrentUserOutputBodySchema = z.strictObject({"account": AccountSchema})
export const RegistrationBeginInputBodySchema = z.strictObject({"display_name": z.string()})
export const AuthenticatorSelectionSchema = z.strictObject({"authenticatorAttachment": z.string().optional(), "requireResidentKey": z.boolean().optional(), "residentKey": z.string().optional(), "userVerification": z.string().optional()})
export const CredentialParameterSchema = z.strictObject({"alg": z.number().refine(Number.isInteger, "Expected integer"), "type": z.string()})
export const RelyingPartyEntitySchema = z.strictObject({"id": z.string(), "name": z.string()})
export const PasskeyUserSchema = z.strictObject({"displayName": z.string(), "id": z.string(), "name": z.string()})
export const PublicKeyCredentialCreationOptionsSchema = z.strictObject({"attestation": z.string().optional(), "attestationFormats": z.array(z.string()).nullable().optional(), "authenticatorSelection": AuthenticatorSelectionSchema.optional(), "challenge": z.string(), "excludeCredentials": z.array(CredentialDescriptorSchema).nullable().optional(), "extensions": AuthenticationExtensionsSchema.optional(), "hints": z.array(z.string()).nullable().optional(), "pubKeyCredParams": z.array(CredentialParameterSchema).nullable().optional(), "rp": RelyingPartyEntitySchema, "timeout": z.number().refine(Number.isInteger, "Expected integer").optional(), "user": PasskeyUserSchema})
export const CredentialCreationSchema = z.strictObject({"mediation": z.string().optional(), "publicKey": PublicKeyCredentialCreationOptionsSchema})
export const PasskeyAttestationSchema = z.strictObject({"attestationObject": z.string(), "authenticatorData": z.string().optional(), "clientDataJSON": z.string(), "publicKey": z.string().optional(), "publicKeyAlgorithm": z.number().refine(Number.isInteger, "Expected integer").optional(), "transports": z.array(z.string()).nullable().optional()})
export const CredentialCreationResponseSchema = z.strictObject({"authenticatorAttachment": z.string().optional(), "clientExtensionResults": AuthenticationExtensionsClientOutputsSchema.optional(), "id": z.string(), "rawId": z.string(), "response": PasskeyAttestationSchema, "type": z.string()})
export const KeyMetadataSchema = z.strictObject({"created_at": z.iso.datetime({ offset: true }), "id": z.string(), "name": z.string(), "public_fingerprint": z.string()})
export const KeysBodySchema = z.strictObject({"keys": z.array(KeyMetadataSchema)})
export const KeyErrorBodySchema = z.strictObject({"error": z.string()})
export const UploadFormSchema = z.strictObject({"name": z.string(), "private_key": z.file()})
export const KeyBodySchema = z.strictObject({"key": KeyMetadataSchema})

export const contracts = {
"beginLogin": { request: {"application/json": LoginBeginInputBodySchema}, responses: {"200": {"application/json": CredentialAssertionSchema}, "400": {"application/json": AuthErrorBodySchema}, "403": {"application/json": AuthErrorBodySchema}, "409": {"application/json": AuthErrorBodySchema}, "415": {"application/json": AuthErrorBodySchema}, "500": {"application/json": AuthErrorBodySchema}, "503": {"application/json": AuthErrorBodySchema}} },
"finishLogin": { request: {"application/json": CredentialAssertionResponseSchema}, responses: {"200": {"application/json": AccountBodySchema}, "400": {"application/json": AuthErrorBodySchema}, "401": {"application/json": AuthErrorBodySchema}, "403": {"application/json": AuthErrorBodySchema}, "409": {"application/json": AuthErrorBodySchema}, "415": {"application/json": AuthErrorBodySchema}, "503": {"application/json": AuthErrorBodySchema}} },
"logout": { request: z.undefined(), responses: {"204": z.undefined(), "403": {"application/json": AuthErrorBodySchema}, "415": {"application/json": AuthErrorBodySchema}, "503": {"application/json": AuthErrorBodySchema}} },
"currentUser": { request: z.undefined(), responses: {"200": {"application/json": CurrentUserOutputBodySchema}, "401": {"application/json": AuthErrorBodySchema}, "403": {"application/json": AuthErrorBodySchema}, "503": {"application/json": AuthErrorBodySchema}} },
"beginRegistration": { request: {"application/json": RegistrationBeginInputBodySchema}, responses: {"200": {"application/json": CredentialCreationSchema}, "400": {"application/json": AuthErrorBodySchema}, "403": {"application/json": AuthErrorBodySchema}, "409": {"application/json": AuthErrorBodySchema}, "415": {"application/json": AuthErrorBodySchema}, "500": {"application/json": AuthErrorBodySchema}, "503": {"application/json": AuthErrorBodySchema}} },
"finishRegistration": { request: {"application/json": CredentialCreationResponseSchema}, responses: {"201": {"application/json": AccountBodySchema}, "400": {"application/json": AuthErrorBodySchema}, "403": {"application/json": AuthErrorBodySchema}, "409": {"application/json": AuthErrorBodySchema}, "415": {"application/json": AuthErrorBodySchema}, "503": {"application/json": AuthErrorBodySchema}} },
"listKeys": { request: z.undefined(), responses: {"200": {"application/json": KeysBodySchema}, "401": {"application/json": KeyErrorBodySchema}, "403": {"application/json": KeyErrorBodySchema}, "503": {"application/json": KeyErrorBodySchema}} },
"uploadKey": { request: {"multipart/form-data": UploadFormSchema}, responses: {"201": {"application/json": KeyBodySchema}, "400": {"application/json": KeyErrorBodySchema}, "401": {"application/json": KeyErrorBodySchema}, "403": {"application/json": KeyErrorBodySchema}, "413": {"application/json": KeyErrorBodySchema}, "415": {"application/json": KeyErrorBodySchema}, "503": {"application/json": KeyErrorBodySchema}} },
"deleteKey": { request: z.undefined(), responses: {"204": z.undefined(), "401": {"application/json": KeyErrorBodySchema}, "403": {"application/json": KeyErrorBodySchema}, "404": {"application/json": KeyErrorBodySchema}, "503": {"application/json": KeyErrorBodySchema}} }
} as const

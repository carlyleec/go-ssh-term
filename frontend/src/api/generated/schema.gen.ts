// Generated from OpenAPI. DO NOT EDIT.
export interface paths {
    "/api/auth/login/begin": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        post: operations["beginLogin"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/auth/login/finish": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        post: operations["finishLogin"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/auth/logout": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        /** End the current browser session */
        post: operations["logout"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/auth/me": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** Get the current account */
        get: operations["currentUser"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/auth/register/begin": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        post: operations["beginRegistration"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/auth/register/finish": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        post: operations["finishRegistration"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/connections": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** List owned saved connections */
        get: operations["listConnections"];
        put?: never;
        /** Create an owned saved connection */
        post: operations["createConnection"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/connections/{id}": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        /** Replace an owned saved connection configuration */
        put: operations["updateConnection"];
        post?: never;
        /** Delete an owned saved connection */
        delete: operations["deleteConnection"];
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/connections/{id}/host-key": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        /** Inspect the next host requiring verification on an owned route */
        post: operations["inspectHost"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/connections/{id}/host-trust": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        /** Approve the exact displayed host fingerprint */
        post: operations["approveHost"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/connections/{id}/host-trust/reset": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        /** Explicitly remove the specified stored fingerprint */
        post: operations["resetHostTrust"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/keys": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** List owned SSH key metadata */
        get: operations["listKeys"];
        put?: never;
        /**
         * Upload an SSH private key
         * @description Streams multipart parts with a 32 KiB total request limit. Duplicate, unknown, and transfer-encoded parts are rejected.
         */
        post: operations["uploadKey"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/keys/{id}": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        post?: never;
        /** Delete an owned SSH key */
        delete: operations["deleteKey"];
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
}
export type webhooks = Record<string, never>;
export interface components {
    schemas: {
        Account: {
            display_name: string;
            id: string;
        };
        AccountBody: {
            account: components["schemas"]["Account"];
        };
        AuthErrorBody: {
            error: string;
        };
        AuthenticationExtensions: {
            appid?: string;
            appidExclude?: string;
            credBlob?: string;
            credProps?: boolean;
            credentialProtectionPolicy?: string;
            enforceCredentialProtectionPolicy?: boolean;
            getCredBlob?: boolean;
            hmacCreateSecret?: boolean;
            hmacGetSecret?: components["schemas"]["HMACGetSecretInputs"];
            largeBlob?: components["schemas"]["LargeBlobInputs"];
            minPinLength?: boolean;
            prf?: components["schemas"]["PRFInputs"];
            remoteClientDataJSON?: string;
            uvm?: boolean;
        };
        AuthenticationExtensionsClientOutputs: {
            appid?: boolean;
            appidExclude?: boolean;
            credProps?: components["schemas"]["CredentialPropertiesOutput"];
            hmacCreateSecret?: boolean;
            hmacGetSecret?: components["schemas"]["HMACGetSecretOutputs"];
            largeBlob?: components["schemas"]["LargeBlobOutputs"];
            prf?: components["schemas"]["PRFOutputs"];
            remoteClientDataJSON?: boolean;
        };
        AuthenticatorAssertionResponse: {
            authenticatorData: string;
            clientDataJSON: string;
            signature: string;
            userHandle?: string;
        };
        AuthenticatorSelection: {
            authenticatorAttachment?: string;
            requireResidentKey?: boolean;
            residentKey?: string;
            userVerification?: string;
        };
        Connection: {
            /** Format: date-time */
            created_at: string;
            host: string;
            id: string;
            /** @description Owned direct connection UUID, when configured */
            jump_connection_id?: string;
            name: string;
            /** Format: int64 */
            port: number;
            ssh_key_id: string;
            /** Format: date-time */
            updated_at: string;
            username: string;
        };
        ConnectionBody: {
            connection: components["schemas"]["Connection"];
        };
        ConnectionErrorBody: {
            error: string;
            /** @description bastion or target */
            hop?: string;
        };
        ConnectionFields: {
            /** @description ASCII DNS hostname or unbracketed IPv4/IPv6 address; no port, zone, or URL */
            host: string;
            /** @description Optional owned direct connection UUID; omit or null for no jump */
            jump_connection_id?: string | null;
            /** @description Trimmed label, 1–64 Unicode characters without controls */
            name: string;
            /** Format: int64 */
            port: number;
            /** @description Canonical UUID of an owned SSH key */
            ssh_key_id: string;
            /** @description Trimmed, 1–64 ASCII letters, digits, underscores, dots or hyphens; starts with a letter, digit or underscore */
            username: string;
        };
        ConnectionsBody: {
            connections: components["schemas"]["Connection"][];
        };
        CredentialAssertion: {
            mediation?: string;
            publicKey: components["schemas"]["PublicKeyCredentialRequestOptions"];
        };
        CredentialAssertionResponse: {
            authenticatorAttachment?: string;
            clientExtensionResults?: components["schemas"]["AuthenticationExtensionsClientOutputs"];
            id: string;
            rawId: string;
            response: components["schemas"]["AuthenticatorAssertionResponse"];
            type: string;
        };
        CredentialCreation: {
            mediation?: string;
            publicKey: components["schemas"]["PublicKeyCredentialCreationOptions"];
        };
        CredentialCreationResponse: {
            authenticatorAttachment?: string;
            clientExtensionResults?: components["schemas"]["AuthenticationExtensionsClientOutputs"];
            id: string;
            rawId: string;
            response: components["schemas"]["PasskeyAttestation"];
            type: string;
        };
        CredentialDescriptor: {
            id: string;
            transports?: string[] | null;
            type: string;
        };
        CredentialParameter: {
            /** Format: int64 */
            alg: number;
            type: string;
        };
        CredentialPropertiesOutput: {
            rk?: boolean;
        };
        CurrentUserOutputBody: {
            account: components["schemas"]["Account"];
        };
        HMACGetSecretInputs: {
            salt1: string;
            salt2?: string;
        };
        HMACGetSecretOutputs: {
            output1?: string;
            output2?: string;
        };
        HostDecision: {
            fingerprint: string;
            host: string;
            /** @description Echo the inspected bastion ID; omit for the target */
            jump_connection_id?: string;
            /** Format: int64 */
            port: number;
        };
        HostInspection: {
            algorithm: string;
            fingerprint: string;
            /** @description bastion or target */
            hop: string;
            host: string;
            /** @description Present only when the fingerprint belongs to the selected bastion */
            jump_connection_id?: string;
            /** Format: int64 */
            port: number;
            /** @description unknown, trusted, or changed */
            state: string;
            trusted_fingerprint: string;
        };
        KeyBody: {
            key: components["schemas"]["KeyMetadata"];
        };
        KeyErrorBody: {
            error: string;
        };
        KeyMetadata: {
            /** Format: date-time */
            created_at: string;
            id: string;
            name: string;
            public_fingerprint: string;
        };
        KeysBody: {
            keys: components["schemas"]["KeyMetadata"][];
        };
        LargeBlobInputs: {
            read?: boolean;
            support?: string;
            write?: string;
        };
        LargeBlobOutputs: {
            blob?: string;
            supported?: boolean;
            written?: boolean;
        };
        LoginBeginInputBody: Record<string, never>;
        PRFInputs: {
            eval?: components["schemas"]["PRFValues"];
            evalByCredential?: {
                [key: string]: components["schemas"]["PRFValues"];
            };
        };
        PRFOutputs: {
            enabled?: boolean;
            results?: components["schemas"]["PRFValues"];
        };
        PRFValues: {
            first: string;
            second?: string;
        };
        PasskeyAttestation: {
            attestationObject: string;
            authenticatorData?: string;
            clientDataJSON: string;
            publicKey?: string;
            /** Format: int64 */
            publicKeyAlgorithm?: number;
            transports?: string[] | null;
        };
        PasskeyUser: {
            displayName: string;
            id: string;
            name: string;
        };
        PublicKeyCredentialCreationOptions: {
            attestation?: string;
            attestationFormats?: string[] | null;
            authenticatorSelection?: components["schemas"]["AuthenticatorSelection"];
            challenge: string;
            excludeCredentials?: components["schemas"]["CredentialDescriptor"][] | null;
            extensions?: components["schemas"]["AuthenticationExtensions"];
            hints?: string[] | null;
            pubKeyCredParams?: components["schemas"]["CredentialParameter"][] | null;
            rp: components["schemas"]["RelyingPartyEntity"];
            /** Format: int64 */
            timeout?: number;
            user: components["schemas"]["PasskeyUser"];
        };
        PublicKeyCredentialRequestOptions: {
            allowCredentials?: components["schemas"]["CredentialDescriptor"][] | null;
            challenge: string;
            extensions?: components["schemas"]["AuthenticationExtensions"];
            hints?: string[] | null;
            rpId?: string;
            /** Format: int64 */
            timeout?: number;
            userVerification?: string;
        };
        RegistrationBeginInputBody: {
            display_name: string;
        };
        RelyingPartyEntity: {
            id: string;
            name: string;
        };
        UploadForm: {
            /** @description Trimmed label: 1–64 Unicode characters without controls; at most 256 raw UTF-8 bytes */
            name: string;
            /**
             * Format: binary
             * @description Unencrypted Ed25519 OpenSSH private key, at most 16 KiB
             */
            private_key: File;
        };
    };
    responses: never;
    parameters: never;
    requestBodies: never;
    headers: never;
    pathItems: never;
}
export type SchemaAccount = components['schemas']['Account'];
export type SchemaAccountBody = components['schemas']['AccountBody'];
export type SchemaAuthErrorBody = components['schemas']['AuthErrorBody'];
export type SchemaAuthenticationExtensions = components['schemas']['AuthenticationExtensions'];
export type SchemaAuthenticationExtensionsClientOutputs = components['schemas']['AuthenticationExtensionsClientOutputs'];
export type SchemaAuthenticatorAssertionResponse = components['schemas']['AuthenticatorAssertionResponse'];
export type SchemaAuthenticatorSelection = components['schemas']['AuthenticatorSelection'];
export type SchemaConnection = components['schemas']['Connection'];
export type SchemaConnectionBody = components['schemas']['ConnectionBody'];
export type SchemaConnectionErrorBody = components['schemas']['ConnectionErrorBody'];
export type SchemaConnectionFields = components['schemas']['ConnectionFields'];
export type SchemaConnectionsBody = components['schemas']['ConnectionsBody'];
export type SchemaCredentialAssertion = components['schemas']['CredentialAssertion'];
export type SchemaCredentialAssertionResponse = components['schemas']['CredentialAssertionResponse'];
export type SchemaCredentialCreation = components['schemas']['CredentialCreation'];
export type SchemaCredentialCreationResponse = components['schemas']['CredentialCreationResponse'];
export type SchemaCredentialDescriptor = components['schemas']['CredentialDescriptor'];
export type SchemaCredentialParameter = components['schemas']['CredentialParameter'];
export type SchemaCredentialPropertiesOutput = components['schemas']['CredentialPropertiesOutput'];
export type SchemaCurrentUserOutputBody = components['schemas']['CurrentUserOutputBody'];
export type SchemaHmacGetSecretInputs = components['schemas']['HMACGetSecretInputs'];
export type SchemaHmacGetSecretOutputs = components['schemas']['HMACGetSecretOutputs'];
export type SchemaHostDecision = components['schemas']['HostDecision'];
export type SchemaHostInspection = components['schemas']['HostInspection'];
export type SchemaKeyBody = components['schemas']['KeyBody'];
export type SchemaKeyErrorBody = components['schemas']['KeyErrorBody'];
export type SchemaKeyMetadata = components['schemas']['KeyMetadata'];
export type SchemaKeysBody = components['schemas']['KeysBody'];
export type SchemaLargeBlobInputs = components['schemas']['LargeBlobInputs'];
export type SchemaLargeBlobOutputs = components['schemas']['LargeBlobOutputs'];
export type SchemaLoginBeginInputBody = components['schemas']['LoginBeginInputBody'];
export type SchemaPrfInputs = components['schemas']['PRFInputs'];
export type SchemaPrfOutputs = components['schemas']['PRFOutputs'];
export type SchemaPrfValues = components['schemas']['PRFValues'];
export type SchemaPasskeyAttestation = components['schemas']['PasskeyAttestation'];
export type SchemaPasskeyUser = components['schemas']['PasskeyUser'];
export type SchemaPublicKeyCredentialCreationOptions = components['schemas']['PublicKeyCredentialCreationOptions'];
export type SchemaPublicKeyCredentialRequestOptions = components['schemas']['PublicKeyCredentialRequestOptions'];
export type SchemaRegistrationBeginInputBody = components['schemas']['RegistrationBeginInputBody'];
export type SchemaRelyingPartyEntity = components['schemas']['RelyingPartyEntity'];
export type SchemaUploadForm = components['schemas']['UploadForm'];
export type $defs = Record<string, never>;
export interface operations {
    beginLogin: {
        parameters: {
            query?: never;
            header: {
                /** @description Configured browser origin */
                Origin: string;
                /** @description application/json, optionally with media-type parameters */
                "Content-Type": string;
            };
            path?: never;
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["LoginBeginInputBody"];
            };
        };
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["CredentialAssertion"];
                };
            };
            /** @description Bad Request */
            400: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["AuthErrorBody"];
                };
            };
            /** @description Forbidden */
            403: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["AuthErrorBody"];
                };
            };
            /** @description Conflict */
            409: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["AuthErrorBody"];
                };
            };
            /** @description Unsupported Media Type */
            415: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["AuthErrorBody"];
                };
            };
            /** @description Internal Server Error */
            500: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["AuthErrorBody"];
                };
            };
            /** @description Service Unavailable */
            503: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["AuthErrorBody"];
                };
            };
        };
    };
    finishLogin: {
        parameters: {
            query?: never;
            header: {
                /** @description Configured browser origin */
                Origin: string;
                /** @description application/json, optionally with media-type parameters */
                "Content-Type": string;
            };
            path?: never;
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["CredentialAssertionResponse"];
            };
        };
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["AccountBody"];
                };
            };
            /** @description Bad Request */
            400: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["AuthErrorBody"];
                };
            };
            /** @description Unauthorized */
            401: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["AuthErrorBody"];
                };
            };
            /** @description Forbidden */
            403: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["AuthErrorBody"];
                };
            };
            /** @description Conflict */
            409: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["AuthErrorBody"];
                };
            };
            /** @description Unsupported Media Type */
            415: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["AuthErrorBody"];
                };
            };
            /** @description Service Unavailable */
            503: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["AuthErrorBody"];
                };
            };
        };
    };
    logout: {
        parameters: {
            query?: never;
            header: {
                /** @description Configured browser origin */
                Origin: string;
                /** @description application/json, optionally with media-type parameters */
                "Content-Type": string;
            };
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description No Content */
            204: {
                headers: {
                    "Cache-Control"?: string;
                    "Set-Cookie"?: string;
                    [name: string]: unknown;
                };
                content?: never;
            };
            /** @description Forbidden */
            403: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["AuthErrorBody"];
                };
            };
            /** @description Unsupported Media Type */
            415: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["AuthErrorBody"];
                };
            };
            /** @description Service Unavailable */
            503: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["AuthErrorBody"];
                };
            };
        };
    };
    currentUser: {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["CurrentUserOutputBody"];
                };
            };
            /** @description Unauthorized */
            401: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["AuthErrorBody"];
                };
            };
            /** @description Forbidden */
            403: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["AuthErrorBody"];
                };
            };
            /** @description Service Unavailable */
            503: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["AuthErrorBody"];
                };
            };
        };
    };
    beginRegistration: {
        parameters: {
            query?: never;
            header: {
                /** @description Configured browser origin */
                Origin: string;
                /** @description application/json, optionally with media-type parameters */
                "Content-Type": string;
            };
            path?: never;
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["RegistrationBeginInputBody"];
            };
        };
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["CredentialCreation"];
                };
            };
            /** @description Bad Request */
            400: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["AuthErrorBody"];
                };
            };
            /** @description Forbidden */
            403: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["AuthErrorBody"];
                };
            };
            /** @description Conflict */
            409: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["AuthErrorBody"];
                };
            };
            /** @description Unsupported Media Type */
            415: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["AuthErrorBody"];
                };
            };
            /** @description Internal Server Error */
            500: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["AuthErrorBody"];
                };
            };
            /** @description Service Unavailable */
            503: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["AuthErrorBody"];
                };
            };
        };
    };
    finishRegistration: {
        parameters: {
            query?: never;
            header: {
                /** @description Configured browser origin */
                Origin: string;
                /** @description application/json, optionally with media-type parameters */
                "Content-Type": string;
            };
            path?: never;
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["CredentialCreationResponse"];
            };
        };
        responses: {
            /** @description Created */
            201: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["AccountBody"];
                };
            };
            /** @description Bad Request */
            400: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["AuthErrorBody"];
                };
            };
            /** @description Forbidden */
            403: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["AuthErrorBody"];
                };
            };
            /** @description Conflict */
            409: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["AuthErrorBody"];
                };
            };
            /** @description Unsupported Media Type */
            415: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["AuthErrorBody"];
                };
            };
            /** @description Service Unavailable */
            503: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["AuthErrorBody"];
                };
            };
        };
    };
    listConnections: {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ConnectionsBody"];
                };
            };
            /** @description Bad Request */
            400: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ConnectionErrorBody"];
                };
            };
            /** @description Unauthorized */
            401: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ConnectionErrorBody"];
                };
            };
            /** @description Forbidden */
            403: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ConnectionErrorBody"];
                };
            };
            /** @description Not Found */
            404: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ConnectionErrorBody"];
                };
            };
            /** @description Conflict */
            409: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ConnectionErrorBody"];
                };
            };
            /** @description Request Entity Too Large */
            413: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ConnectionErrorBody"];
                };
            };
            /** @description Unsupported Media Type */
            415: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ConnectionErrorBody"];
                };
            };
            /** @description Service Unavailable */
            503: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ConnectionErrorBody"];
                };
            };
        };
    };
    createConnection: {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["ConnectionFields"];
            };
        };
        responses: {
            /** @description Created */
            201: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ConnectionBody"];
                };
            };
            /** @description Bad Request */
            400: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ConnectionErrorBody"];
                };
            };
            /** @description Unauthorized */
            401: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ConnectionErrorBody"];
                };
            };
            /** @description Forbidden */
            403: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ConnectionErrorBody"];
                };
            };
            /** @description Not Found */
            404: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ConnectionErrorBody"];
                };
            };
            /** @description Conflict */
            409: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ConnectionErrorBody"];
                };
            };
            /** @description Request Entity Too Large */
            413: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ConnectionErrorBody"];
                };
            };
            /** @description Unsupported Media Type */
            415: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ConnectionErrorBody"];
                };
            };
            /** @description Service Unavailable */
            503: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ConnectionErrorBody"];
                };
            };
        };
    };
    updateConnection: {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: string;
            };
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["ConnectionFields"];
            };
        };
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ConnectionBody"];
                };
            };
            /** @description Bad Request */
            400: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ConnectionErrorBody"];
                };
            };
            /** @description Unauthorized */
            401: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ConnectionErrorBody"];
                };
            };
            /** @description Forbidden */
            403: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ConnectionErrorBody"];
                };
            };
            /** @description Not Found */
            404: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ConnectionErrorBody"];
                };
            };
            /** @description Conflict */
            409: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ConnectionErrorBody"];
                };
            };
            /** @description Request Entity Too Large */
            413: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ConnectionErrorBody"];
                };
            };
            /** @description Unsupported Media Type */
            415: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ConnectionErrorBody"];
                };
            };
            /** @description Service Unavailable */
            503: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ConnectionErrorBody"];
                };
            };
        };
    };
    deleteConnection: {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: string;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description No Content */
            204: {
                headers: {
                    [name: string]: unknown;
                };
                content?: never;
            };
            /** @description Bad Request */
            400: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ConnectionErrorBody"];
                };
            };
            /** @description Unauthorized */
            401: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ConnectionErrorBody"];
                };
            };
            /** @description Forbidden */
            403: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ConnectionErrorBody"];
                };
            };
            /** @description Not Found */
            404: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ConnectionErrorBody"];
                };
            };
            /** @description Conflict */
            409: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ConnectionErrorBody"];
                };
            };
            /** @description Request Entity Too Large */
            413: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ConnectionErrorBody"];
                };
            };
            /** @description Unsupported Media Type */
            415: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ConnectionErrorBody"];
                };
            };
            /** @description Service Unavailable */
            503: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ConnectionErrorBody"];
                };
            };
        };
    };
    inspectHost: {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: string;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HostInspection"];
                };
            };
            /** @description Bad Request */
            400: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ConnectionErrorBody"];
                };
            };
            /** @description Unauthorized */
            401: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ConnectionErrorBody"];
                };
            };
            /** @description Forbidden */
            403: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ConnectionErrorBody"];
                };
            };
            /** @description Not Found */
            404: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ConnectionErrorBody"];
                };
            };
            /** @description Conflict */
            409: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ConnectionErrorBody"];
                };
            };
            /** @description Request Entity Too Large */
            413: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ConnectionErrorBody"];
                };
            };
            /** @description Unsupported Media Type */
            415: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ConnectionErrorBody"];
                };
            };
            /** @description Bad Gateway */
            502: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ConnectionErrorBody"];
                };
            };
            /** @description Service Unavailable */
            503: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ConnectionErrorBody"];
                };
            };
            /** @description Gateway Timeout */
            504: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ConnectionErrorBody"];
                };
            };
        };
    };
    approveHost: {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: string;
            };
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["HostDecision"];
            };
        };
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HostInspection"];
                };
            };
            /** @description Bad Request */
            400: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ConnectionErrorBody"];
                };
            };
            /** @description Unauthorized */
            401: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ConnectionErrorBody"];
                };
            };
            /** @description Forbidden */
            403: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ConnectionErrorBody"];
                };
            };
            /** @description Not Found */
            404: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ConnectionErrorBody"];
                };
            };
            /** @description Conflict */
            409: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ConnectionErrorBody"];
                };
            };
            /** @description Request Entity Too Large */
            413: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ConnectionErrorBody"];
                };
            };
            /** @description Unsupported Media Type */
            415: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ConnectionErrorBody"];
                };
            };
            /** @description Bad Gateway */
            502: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ConnectionErrorBody"];
                };
            };
            /** @description Service Unavailable */
            503: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ConnectionErrorBody"];
                };
            };
            /** @description Gateway Timeout */
            504: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ConnectionErrorBody"];
                };
            };
        };
    };
    resetHostTrust: {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: string;
            };
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["HostDecision"];
            };
        };
        responses: {
            /** @description No Content */
            204: {
                headers: {
                    [name: string]: unknown;
                };
                content?: never;
            };
            /** @description Bad Request */
            400: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ConnectionErrorBody"];
                };
            };
            /** @description Unauthorized */
            401: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ConnectionErrorBody"];
                };
            };
            /** @description Forbidden */
            403: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ConnectionErrorBody"];
                };
            };
            /** @description Not Found */
            404: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ConnectionErrorBody"];
                };
            };
            /** @description Conflict */
            409: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ConnectionErrorBody"];
                };
            };
            /** @description Request Entity Too Large */
            413: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ConnectionErrorBody"];
                };
            };
            /** @description Unsupported Media Type */
            415: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ConnectionErrorBody"];
                };
            };
            /** @description Bad Gateway */
            502: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ConnectionErrorBody"];
                };
            };
            /** @description Service Unavailable */
            503: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ConnectionErrorBody"];
                };
            };
            /** @description Gateway Timeout */
            504: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ConnectionErrorBody"];
                };
            };
        };
    };
    listKeys: {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description OK */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["KeysBody"];
                };
            };
            /** @description Unauthorized */
            401: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["KeyErrorBody"];
                };
            };
            /** @description Forbidden */
            403: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["KeyErrorBody"];
                };
            };
            /** @description Service Unavailable */
            503: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["KeyErrorBody"];
                };
            };
        };
    };
    uploadKey: {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody: {
            content: {
                "multipart/form-data": components["schemas"]["UploadForm"];
            };
        };
        responses: {
            /** @description Created */
            201: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["KeyBody"];
                };
            };
            /** @description Bad Request */
            400: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["KeyErrorBody"];
                };
            };
            /** @description Unauthorized */
            401: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["KeyErrorBody"];
                };
            };
            /** @description Forbidden */
            403: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["KeyErrorBody"];
                };
            };
            /** @description Request Entity Too Large */
            413: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["KeyErrorBody"];
                };
            };
            /** @description Unsupported Media Type */
            415: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["KeyErrorBody"];
                };
            };
            /** @description Service Unavailable */
            503: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["KeyErrorBody"];
                };
            };
        };
    };
    deleteKey: {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: string;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description No Content */
            204: {
                headers: {
                    [name: string]: unknown;
                };
                content?: never;
            };
            /** @description Unauthorized */
            401: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["KeyErrorBody"];
                };
            };
            /** @description Forbidden */
            403: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["KeyErrorBody"];
                };
            };
            /** @description Not Found */
            404: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["KeyErrorBody"];
                };
            };
            /** @description Conflict */
            409: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["KeyErrorBody"];
                };
            };
            /** @description Service Unavailable */
            503: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["KeyErrorBody"];
                };
            };
        };
    };
}

import 'dart:convert';

import 'package:cryptography/cryptography.dart';

import 'package:byr_client/shared/core/constants.dart';
import 'package:byr_client/shared/core/result.dart';
import 'package:byr_client/shared/data/models/encrypted_data.dart';
import 'package:byr_client/shared/data/api/api_base.dart';
import 'package:byr_client/shared/data/api/api_client.dart';

/// Plaintext envelope returned by the experimental encrypted endpoint.
class DecryptedPayload {
  final dynamic data;

  const DecryptedPayload(this.data);
}

/// Experimental `/exp` API.
///
/// Recovered flow (`CryptoSolver::decrypt` + `Solver::cryptoX25519`):
/// 1. generate an ephemeral X25519 key pair;
/// 2. GET `/exp/status?key=<base64(publicKey)>`;
/// 3. verify `sign` with the bundled Ed25519 key over `content`;
/// 4. derive the shared secret with the server `public_key`;
/// 5. AES-GCM decrypt `content` (nonce 12 bytes, MAC 16 bytes).
class CryptoApi extends BaseApi {
  CryptoApi(super.client);

  final X25519 _x25519 = X25519();
  final Ed25519 _ed25519 = Ed25519();
  final AesGcm _aesGcm = AesGcm.with256bits();

  Future<Result<DecryptedPayload>> testExperimental() async {
    try {
      final keyPair = await _x25519.newKeyPair();
      final publicKey = await keyPair.extractPublicKey();
      final json = await client.getAbsolute(
        '${Constants.exApiUrl}/status',
        query: {'key': base64Encode(publicKey.bytes)},
      );

      final envelope = EncryptedData.fromJson(json);
      if (envelope.code != null &&
          envelope.code != '0' &&
          envelope.code != '0000') {
        return Result.failure(envelope.msg ?? '请求失败');
      }

      final content = envelope.content;
      final serverKey = envelope.publicKey;
      final sign = envelope.sign;
      if (content == null || serverKey == null || sign == null) {
        return const Result.failure('加密响应字段缺失');
      }

      final contentBytes = base64Decode(content);
      final verified = await _ed25519.verify(
        contentBytes,
        signature: Signature(
          base64Decode(sign),
          publicKey: SimplePublicKey(
            base64Decode(Constants.ed25519PublicKey),
            type: KeyPairType.ed25519,
          ),
        ),
      );
      if (!verified) {
        return const Result.failure('签名验证失败');
      }

      final sharedSecret = await _x25519.sharedSecretKey(
        keyPair: keyPair,
        remotePublicKey: SimplePublicKey(
          base64Decode(serverKey),
          type: KeyPairType.x25519,
        ),
      );

      final box = SecretBox.fromConcatenation(
        contentBytes,
        nonceLength: 12,
        macLength: 16,
      );
      final plain = await _aesGcm.decrypt(box, secretKey: sharedSecret);

      final text = utf8.decode(plain);
      dynamic decoded;
      try {
        decoded = jsonDecode(text);
      } catch (_) {
        decoded = text;
      }
      return Result.success(DecryptedPayload(decoded));
    } on ApiException catch (error) {
      return Result.failure(error.message);
    } catch (error) {
      return Result.failure(error.toString());
    }
  }
}

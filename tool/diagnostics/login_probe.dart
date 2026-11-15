// Manual live probe against the real BYR endpoints (not shipped in the app).
// ignore_for_file: avoid_print

import 'package:byr_client/shared/data/api/api_client.dart';
import 'package:byr_client/shared/data/api/auth_api.dart';

Future<void> main() async {
  final auth = AuthApi(ApiClient());
  final result = await auth.login('test', 'test');
  print('isFailure=${result.isFailure}');
  print('message=${result.message}');
  final web = await auth.webLoginCheck('test', 'test');
  print('web.ok=${web.ok} code=${web.code} message=${web.message}');
  print('web.raw=${web.raw}');
}

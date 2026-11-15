import 'dart:convert';
import 'dart:typed_data';

import 'package:dio/dio.dart';

import 'package:byr_client/shared/data/api/api_client.dart';
import 'package:byr_client/shared/services/device_services.dart';
import 'package:byr_client/shared/storage/preference_store.dart';

class MemoryPreferences implements PreferenceStore {
  final Map<String, Object> values = {};

  @override
  Future<String?> readString(String key) async => values[key] as String?;

  @override
  Future<int?> readInt(String key) async => values[key] as int?;

  @override
  Future<double?> readDouble(String key) async => values[key] as double?;

  @override
  Future<bool?> readBool(String key) async => values[key] as bool?;

  @override
  Future<void> write(String key, Object value) async => values[key] = value;

  @override
  Future<void> remove(String key) async => values.remove(key);
}

class FakeDeviceServices implements DeviceServices {
  String? copiedText;
  Uri? openedLink;
  List<SelectedAttachment> attachments = [];
  int pickerCalls = 0;

  @override
  Future<void> copyText(String text) async => copiedText = text;

  @override
  Future<bool> openLink(Uri uri) async {
    openedLink = uri;
    return true;
  }

  @override
  Future<List<SelectedAttachment>> pickAttachments() async {
    pickerCalls++;
    return attachments;
  }
}

class FakeForumServer implements HttpClientAdapter {
  FakeForumServer([this.responses = const {}]);

  final Map<String, Object> responses;
  final List<RequestOptions> requests = [];

  ApiClient createClient() {
    final dio = Dio(BaseOptions(responseType: ResponseType.plain));
    dio.httpClientAdapter = this;
    return ApiClient(dio: dio);
  }

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    requests.add(options);
    return ResponseBody.fromString(
      jsonEncode(responses[options.uri.path] ?? const {}),
      200,
      headers: {
        Headers.contentTypeHeader: ['application/json'],
      },
    );
  }

  @override
  void close({bool force = false}) {}
}

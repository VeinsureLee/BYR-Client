import 'package:byr_client/shared/core/result.dart';
import 'package:byr_client/shared/data/models/mail_box.dart';
import 'package:byr_client/shared/data/models/status.dart';
import 'package:byr_client/shared/data/api/api_base.dart';

/// Mail endpoints (`_MailService`).
class MailApi extends BaseApi {
  MailApi(super.client);

  Future<Result<MailBox>> getMailBox({
    required String box,
    int page = 1,
    int count = 50,
  }) => guard(
    () => client.get(
      '/mail/$box.json',
      query: {'page': '$page', 'count': '$count'},
    ),
    MailBox.fromJson,
  );

  /// `/mail/{box}/{id}.json`
  Future<Result<Mail>> getIndexMail({
    required String box,
    required String id,
  }) => guard(() => client.get('/mail/$box/$id.json'), Mail.fromJson);

  Future<Result<Status>> sendMail({
    required String id,
    required String content,
    String title = '',
  }) => guard(
    () => client.postForm(
      '/mail/send.json',
      form: {
        'id': id,
        'title': title,
        'content': content,
        'signature': '0',
        'backup': '1',
      },
    ),
    Status.fromJson,
  );

  /// `/mail/{box}/reply/{id}.json`
  Future<Result<Mail>> replyMail({
    required String box,
    required String id,
    required String content,
    String title = '',
  }) => guard(
    () => client.postForm(
      '/mail/$box/reply/$id.json',
      form: {
        'title': title,
        'content': content,
        'signature': '0',
        'backup': '1',
      },
    ),
    Mail.fromJson,
  );

  /// `/mail/{box}/forward/{id}.json`
  ///
  /// The original form only contains `target`, `noansi=0` and `big5=0`.
  Future<Result<Mail>> forwardMail({
    required String box,
    required String id,
    required String target,
  }) => guard(
    () => client.postForm(
      '/mail/$box/forward/$id.json',
      form: {'target': target, 'noansi': '0', 'big5': '0'},
    ),
    Mail.fromJson,
  );

  /// `/mail/{box}/delete/{id}.json`
  Future<Result<Mail>> deleteMail({required String box, required String id}) =>
      guard(() => client.postForm('/mail/$box/delete/$id.json'), Mail.fromJson);
}

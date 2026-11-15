import 'package:byr_client/shared/core/result.dart';
import 'package:byr_client/shared/data/models/root_section.dart';
import 'package:byr_client/shared/data/models/section.dart';
import 'package:byr_client/shared/data/api/api_base.dart';

/// Section endpoints (`_SectionService`).
class SectionApi extends BaseApi {
  SectionApi(super.client);

  Future<Result<RootSection>> getRootSection() =>
      guard(() => client.get('/section.json'), RootSection.fromJson);

  Future<Result<Section>> getSection(String section) =>
      guard(() => client.get('/section/$section.json'), Section.fromJson);
}

import 'package:material_ui/material_ui.dart';

import 'package:byr_client/app/app_scope.dart';
import 'package:byr_client/shared/data/models/board.dart';
import 'package:byr_client/shared/data/models/root_section.dart';
import 'package:byr_client/features/compose/compose_screen.dart';

/// Opens the global "new post" flow: choose a section, then a board, then the
/// composer. Returns `true` when a post was published.
///
/// This is shared by the mobile bottom navigation and the desktop rail so the
/// compose entry is never tied to a board page.
Future<bool> openComposeFlow(BuildContext context) async {
  final api = AppScope.of(context).api;
  final root = await api.section.getRootSection();
  if (!context.mounted) return false;
  if (root.isFailure || root.data == null) {
    ScaffoldMessenger.of(context)
        .showSnackBar(SnackBar(content: Text(root.message ?? '加载分区失败')));
    return false;
  }

  final section = await showDialog<SectionElement>(
    context: context,
    builder: (context) => AlertDialog(
      title: const Text('选择分区'),
      content: SizedBox(
        width: double.maxFinite,
        height: 400,
        child: ListView(
          children: [
            for (final item in root.data!.sections)
              ListTile(
                title: Text(
                  item.description?.isNotEmpty == true
                      ? item.description!
                      : (item.name ?? '未命名'),
                ),
                onTap: () => Navigator.of(context).pop(item),
              ),
          ],
        ),
      ),
      actions: [
        TextButton(
          onPressed: () => Navigator.of(context).pop(),
          child: const Text('取消'),
        ),
      ],
    ),
  );
  if (section?.name == null || !context.mounted) return false;

  final detail = await api.section.getSection(section!.name!);
  if (!context.mounted) return false;
  if (detail.isFailure || detail.data == null) {
    ScaffoldMessenger.of(context)
        .showSnackBar(SnackBar(content: Text(detail.message ?? '加载版面失败')));
    return false;
  }
  final boards = <Board>[
    ...detail.data!.boards,
    for (final sub in detail.data!.subSections) ...sub.boards,
  ];

  final board = await showDialog<Board>(
    context: context,
    builder: (context) => AlertDialog(
      title: const Text('选择版面'),
      content: SizedBox(
        width: double.maxFinite,
        height: 400,
        child: ListView(
          children: [
            for (final item in boards)
              ListTile(
                title: Text(
                  item.description?.isNotEmpty == true
                      ? item.description!
                      : (item.name ?? '未命名'),
                ),
                subtitle: Text(item.name ?? ''),
                onTap: () => Navigator.of(context).pop(item),
              ),
          ],
        ),
      ),
      actions: [
        TextButton(
          onPressed: () => Navigator.of(context).pop(),
          child: const Text('取消'),
        ),
      ],
    ),
  );
  if (board?.name == null || !context.mounted) return false;

  final posted = await Navigator.of(context).push<bool>(
    MaterialPageRoute(builder: (_) => ComposeScreen(boardName: board!.name!)),
  );
  if (posted == true && context.mounted) {
    ScaffoldMessenger.of(context)
        .showSnackBar(const SnackBar(content: Text('发布成功')));
  }
  return posted == true;
}

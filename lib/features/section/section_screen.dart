import 'package:material_ui/material_ui.dart';

import 'package:byr_client/app/app_scope.dart';
import 'package:byr_client/shared/core/result.dart';
import 'package:byr_client/shared/data/models/board.dart';
import 'package:byr_client/shared/data/models/section.dart';
import 'package:byr_client/shared/widgets/common.dart';
import 'package:byr_client/features/board/board_screen.dart';

class SectionScreen extends StatefulWidget {
  const SectionScreen({
    super.key,
    required this.sectionName,
    this.sectionTitle,
  });

  final String sectionName;
  final String? sectionTitle;

  @override
  State<SectionScreen> createState() => _SectionScreenState();
}

class _SectionScreenState extends State<SectionScreen> {
  Future<Result<Section>>? _future;

  @override
  void initState() {
    super.initState();
    _load();
  }

  void _load() {
    final future = AppScope.of(context).api.section
        .getSection(widget.sectionName);
    setState(() {
      _future = future;
    });
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: Text(widget.sectionTitle ?? widget.sectionName)),
      body: FutureBuilder<Result<Section>>(
        future: _future,
        builder: (context, snapshot) {
          if (snapshot.connectionState != ConnectionState.done) {
            return const LoadingView();
          }
          return ResultView<Section>(
            result: snapshot.data,
            onRetry: _load,
            builder: (section) {
              final boards = section.boards;
              final subs = section.subSections;
              if (boards.isEmpty && subs.isEmpty) {
                return const EmptyView();
              }
              return ListView(
                children: [
                  for (final board in boards) _boardTile(board),
                  for (final sub in subs)
                    ExpansionTile(
                      title: Text(
                        sub.description?.isNotEmpty == true
                            ? sub.description!
                            : (sub.name ?? '子分区'),
                      ),
                      children: [
                        for (final board in sub.boards) _boardTile(board),
                      ],
                    ),
                ],
              );
            },
          );
        },
      ),
    );
  }

  Widget _boardTile(Board board) {
    final name = board.name;
    if (name == null) return const SizedBox.shrink();
    final title = board.description?.isNotEmpty == true
        ? board.description!
        : name;
    return ListTile(
      title: Text(title),
      subtitle: Text(name, maxLines: 1, overflow: TextOverflow.ellipsis),
      trailing: const Icon(Icons.chevron_right),
      onTap: () {
        Navigator.of(
          context,
        ).push(MaterialPageRoute(builder: (_) => BoardScreen(boardName: name)));
      },
    );
  }
}

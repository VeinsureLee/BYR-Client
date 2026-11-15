import 'package:material_ui/material_ui.dart';

import 'package:byr_client/app/app_scope.dart';
import 'package:byr_client/shared/data/models/board.dart';
import 'package:byr_client/shared/data/models/root_section.dart';
import 'package:byr_client/shared/widgets/common.dart';
import 'package:byr_client/features/board/board_screen.dart';

class _SectionGroup {
  const _SectionGroup({required this.section, required this.boards});

  final SectionElement section;
  final List<Board> boards;
}

/// All boards grouped by root section, with a local name filter.
class AllBoardsScreen extends StatefulWidget {
  const AllBoardsScreen({super.key});

  @override
  State<AllBoardsScreen> createState() => _AllBoardsScreenState();
}

class _AllBoardsScreenState extends State<AllBoardsScreen> {
  final _searchController = TextEditingController();
  List<_SectionGroup>? _groups;
  String? _error;
  String _query = '';

  @override
  void initState() {
    super.initState();
    _load();
  }

  @override
  void dispose() {
    _searchController.dispose();
    super.dispose();
  }

  Future<void> _load() async {
    setState(() {
      _error = null;
      _groups = null;
    });
    final api = AppScope.of(context).api;
    final root = await api.section.getRootSection();
    if (!mounted) return;
    if (root.isFailure || root.data == null) {
      setState(() => _error = root.message ?? '加载分区失败');
      return;
    }
    final sections = root.data!.sections;
    final details = await Future.wait(
      sections.map((section) {
        final name = section.name;
        if (name == null) return Future.value(null);
        return api.section.getSection(name);
      }),
    );
    if (!mounted) return;
    final groups = <_SectionGroup>[];
    for (var i = 0; i < sections.length; i++) {
      final detail = details[i];
      final section = detail?.data;
      if (section == null) continue;
      final boards = <Board>[
        ...section.boards,
        for (final sub in section.subSections) ...sub.boards,
      ];
      groups.add(_SectionGroup(section: sections[i], boards: boards));
    }
    setState(() => _groups = groups);
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: const Text('全部版面')),
      body: Column(
        children: [
          Padding(
            padding: const EdgeInsets.all(12),
            child: TextField(
              controller: _searchController,
              onChanged: (value) =>
                  setState(() => _query = value.trim().toLowerCase()),
              decoration: const InputDecoration(
                hintText: '筛选版面名称',
                prefixIcon: Icon(Icons.search),
                border: OutlineInputBorder(),
                isDense: true,
              ),
            ),
          ),
          Expanded(child: _buildBody()),
        ],
      ),
    );
  }

  Widget _buildBody() {
    final groups = _groups;
    if (_error != null) {
      return ErrorView(message: _error!, onRetry: _load);
    }
    if (groups == null) return const LoadingView();

    final query = _query;
    final filtered = <_SectionGroup>[];
    for (final group in groups) {
      final boards = group.boards.where((board) {
        if (query.isEmpty) return true;
        final name = (board.name ?? '').toLowerCase();
        final desc = (board.description ?? '').toLowerCase();
        return name.contains(query) || desc.contains(query);
      }).toList();
      if (boards.isNotEmpty) {
        filtered.add(_SectionGroup(section: group.section, boards: boards));
      }
    }
    if (filtered.isEmpty) return const EmptyView(message: '没有匹配的版面');

    return ListView(
      children: [
        for (final group in filtered)
          ExpansionTile(
            initiallyExpanded: query.isNotEmpty,
            title: Text(
              group.section.description?.isNotEmpty == true
                  ? group.section.description!
                  : (group.section.name ?? '分区'),
            ),
            subtitle: Text('${group.boards.length} 个版面'),
            children: [for (final board in group.boards) _boardTile(board)],
          ),
      ],
    );
  }

  Widget _boardTile(Board board) {
    final name = board.name;
    if (name == null) return const SizedBox.shrink();
    return ListTile(
      leading: const Icon(Icons.forum_outlined),
      title: Text(
        board.description?.isNotEmpty == true ? board.description! : name,
      ),
      subtitle: Text(name),
      trailing: const Icon(Icons.chevron_right),
      onTap: () => Navigator.of(
        context,
      ).push(MaterialPageRoute(builder: (_) => BoardScreen(boardName: name))),
    );
  }
}

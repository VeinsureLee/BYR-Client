import 'package:material_ui/material_ui.dart';

import 'package:byr_client/app/app_scope.dart';
import 'package:byr_client/shared/core/result.dart';
import 'package:byr_client/shared/data/models/board.dart';
import 'package:byr_client/shared/data/models/search.dart';
import 'package:byr_client/shared/widgets/common.dart';
import 'package:byr_client/features/board/board_screen.dart';

/// Board name search (`/open/search/board.json`).
class BoardSearchScreen extends StatefulWidget {
  const BoardSearchScreen({super.key});

  @override
  State<BoardSearchScreen> createState() => _BoardSearchScreenState();
}

class _BoardSearchScreenState extends State<BoardSearchScreen> {
  final _controller = TextEditingController();
  Future<Result<SearchBoard>>? _future;
  String _lastQuery = '';

  @override
  void dispose() {
    _controller.dispose();
    super.dispose();
  }

  void _search() {
    final query = _controller.text.trim();
    if (query.isEmpty) return;
    setState(() {
      _lastQuery = query;
      _future = AppScope.of(context).api.search.searchBoard(board: query);
    });
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: TextField(
          controller: _controller,
          autofocus: true,
          textInputAction: TextInputAction.search,
          onSubmitted: (_) => _search(),
          decoration: const InputDecoration(
            hintText: '搜索版面名称',
            border: InputBorder.none,
          ),
        ),
        actions: [
          IconButton(onPressed: _search, icon: const Icon(Icons.search)),
        ],
      ),
      body: _future == null
          ? const EmptyView(message: '输入关键词搜索版面')
          : FutureBuilder<Result<SearchBoard>>(
              future: _future,
              builder: (context, snapshot) {
                if (snapshot.connectionState != ConnectionState.done) {
                  return const LoadingView();
                }
                return ResultView<SearchBoard>(
                  result: snapshot.data,
                  onRetry: _search,
                  builder: (data) {
                    if (data.boards.isEmpty) {
                      return EmptyView(message: '没有找到与“$_lastQuery”相关的版面');
                    }
                    return ListView.separated(
                      itemCount: data.boards.length,
                      separatorBuilder: (_, _) => const Divider(height: 1),
                      itemBuilder: (context, index) {
                        final board = data.boards[index];
                        return _boardTile(board);
                      },
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

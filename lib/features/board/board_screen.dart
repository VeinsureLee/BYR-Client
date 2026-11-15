import 'package:material_ui/material_ui.dart';

import 'package:byr_client/app/app_scope.dart';
import 'package:byr_client/shared/core/result.dart';
import 'package:byr_client/shared/core/time_format.dart';
import 'package:byr_client/shared/data/models/board.dart';
import 'package:byr_client/shared/data/models/user.dart';
import 'package:byr_client/shared/widgets/common.dart';
import 'package:byr_client/shared/widgets/user_avatar.dart';
import 'package:byr_client/features/thread/thread_screen.dart';

class BoardScreen extends StatefulWidget {
  const BoardScreen({super.key, required this.boardName});

  final String boardName;

  @override
  State<BoardScreen> createState() => _BoardScreenState();
}

class _BoardScreenState extends State<BoardScreen> {
  Future<Result<Board>>? _future;
  Board? _board;
  int _page = 1;
  bool? _favoriteOverride;
  bool _favoriting = false;

  bool get _isFavorite => _favoriteOverride ?? _board?.isFavorite ?? false;

  @override
  void initState() {
    super.initState();
    _load();
  }

  void _load() {
    final future = AppScope.of(context).api.board
        .getBoard(board: widget.boardName, page: _page);
    future.then((result) {
      if (!mounted || result.data == null) return;
      final current = result.data!.pagination?.pageCurrentCount;
      setState(() {
        _board = result.data;
        // The server clamps an out-of-range page back to the last page.
        // Keep the pager label in sync with the data that was returned.
        if (current != null && current > 0 && current != _page) {
          _page = current;
        }
      });
    });
    setState(() {
      _future = future;
    });
  }

  void _changePage(int delta) {
    final next = _page + delta;
    if (next < 1) return;
    final total = _board?.pagination?.pageAllCount;
    if (total != null && total > 0 && next > total) return;
    setState(() => _page = next);
    _load();
  }

  Future<void> _toggleFavorite() async {
    if (_favoriting) return;
    final wasFavorite = _isFavorite;
    setState(() => _favoriting = true);
    final api = AppScope.of(context).api;
    final result = wasFavorite
        ? await api.favorite.removeFavorite(widget.boardName)
        : await api.favorite.addFavorite(widget.boardName);
    if (!mounted) return;
    final message = result.message ?? '';
    setState(() {
      _favoriting = false;
      if (result.isSuccess) {
        _favoriteOverride = !wasFavorite;
      } else if (message.contains('已经收藏') || message.contains('已收藏')) {
        _favoriteOverride = true;
      }
    });
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(
        content: Text(
          result.isSuccess
              ? (wasFavorite ? '已取消收藏版面' : '已收藏版面')
              : (message.isEmpty ? '操作失败' : message),
        ),
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: Text(
          _board?.description?.isNotEmpty == true
              ? _board!.description!
              : widget.boardName,
        ),
        actions: [
          IconButton(
            onPressed: _favoriting || _board == null ? null : _toggleFavorite,
            icon: AnimatedSwitcher(
              duration: const Duration(milliseconds: 220),
              switchInCurve: Curves.easeOutBack,
              transitionBuilder: (child, animation) =>
                  ScaleTransition(scale: animation, child: child),
              child: Icon(
                _isFavorite ? Icons.star : Icons.star_border,
                key: ValueKey<bool>(_isFavorite),
                color: _isFavorite ? Colors.amber : null,
              ),
            ),
            tooltip: _isFavorite ? '取消收藏版面' : '收藏版面',
          ),
        ],
      ),
      body: FutureBuilder<Result<Board>>(
        future: _future,
        builder: (context, snapshot) {
          if (snapshot.connectionState != ConnectionState.done) {
            return const LoadingView();
          }
          return ResultView<Board>(
            result: snapshot.data,
            onRetry: _load,
            builder: (board) {
              final articles = board.articles;
              return Column(
                children: [
                  Expanded(
                    child: articles.isEmpty
                        ? const EmptyView()
                        : RefreshIndicator(
                            onRefresh: () async => _load(),
                            child: ListView.separated(
                              itemCount: articles.length,
                              separatorBuilder: (_, _) =>
                                  const Divider(height: 1),
                              itemBuilder: (context, index) {
                                final article = articles[index];
                                final time = formatTimestamp(article.postTime);
                                return ListTile(
                                  leading: UserAvatar(
                                    user: article.user,
                                    radius: 16,
                                    onTap: () =>
                                        openUserProfile(context, article.user),
                                  ),
                                  title: Text(
                                    article.title ?? '无标题',
                                    maxLines: 2,
                                    overflow: TextOverflow.ellipsis,
                                  ),
                                  subtitle: Text(
                                    '${displayUserName(article.user, fallback: '')}'
                                    '${time.isEmpty ? '' : ' · $time'}'
                                    ' · ${article.replyCount ?? 0} 回复',
                                    maxLines: 1,
                                    overflow: TextOverflow.ellipsis,
                                  ),
                                  trailing: article.isTop == true
                                      ? const Icon(Icons.push_pin, size: 18)
                                      : null,
                                  onTap: () {
                                    final id = article.id;
                                    if (id == null) return;
                                    Navigator.of(context).push(
                                      MaterialPageRoute(
                                        builder: (_) => ThreadScreen(
                                          boardName: widget.boardName,
                                          threadId: id,
                                          fromBoard: true,
                                        ),
                                      ),
                                    );
                                  },
                                );
                              },
                            ),
                          ),
                  ),
                  _Pager(
                    page: _page,
                    total: board.pagination?.pageAllCount,
                    onChanged: _changePage,
                  ),
                ],
              );
            },
          );
        },
      ),
    );
  }
}

class _Pager extends StatelessWidget {
  const _Pager({
    required this.page,
    required this.total,
    required this.onChanged,
  });

  final int page;
  final int? total;
  final void Function(int delta) onChanged;

  @override
  Widget build(BuildContext context) {
    final totalPages = total;
    final hasNext = totalPages == null || totalPages <= 0 || page < totalPages;
    return SafeArea(
      top: false,
      child: Padding(
        padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 8),
        child: Row(
          mainAxisAlignment: MainAxisAlignment.spaceBetween,
          children: [
            TextButton.icon(
              onPressed: page > 1 ? () => onChanged(-1) : null,
              icon: const Icon(Icons.chevron_left),
              label: const Text('上一页'),
            ),
            Text(
              totalPages == null || totalPages <= 0
                  ? '第 $page 页'
                  : '第 $page / $totalPages 页',
            ),
            TextButton.icon(
              onPressed: hasNext ? () => onChanged(1) : null,
              icon: const Icon(Icons.chevron_right),
              label: const Text('下一页'),
            ),
          ],
        ),
      ),
    );
  }
}

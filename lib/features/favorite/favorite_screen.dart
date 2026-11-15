import 'package:material_ui/material_ui.dart';

import 'package:byr_client/app/app_scope.dart';
import 'package:byr_client/shared/core/result.dart';
import 'package:byr_client/shared/data/models/board.dart';
import 'package:byr_client/shared/data/models/collection.dart';
import 'package:byr_client/shared/data/models/favorite.dart';
import 'package:byr_client/shared/data/models/favor_post.dart';
import 'package:byr_client/shared/data/models/user.dart';
import 'package:byr_client/shared/widgets/common.dart';
import 'package:byr_client/shared/widgets/user_avatar.dart';
import 'package:byr_client/features/board/board_screen.dart';
import 'package:byr_client/features/thread/thread_screen.dart';

/// 收藏夹：版面收藏 + 文章收藏（/collection）+ 帖子收藏（/favorpost）。
class FavoriteScreen extends StatelessWidget {
  const FavoriteScreen({super.key});

  @override
  Widget build(BuildContext context) {
    return DefaultTabController(
      length: 3,
      child: Scaffold(
        appBar: AppBar(
          title: const Text('收藏夹'),
          bottom: const TabBar(
            tabs: [
              Tab(text: '文章'),
              Tab(text: '帖子'),
              Tab(text: '版面'),
            ],
          ),
        ),
        body: const TabBarView(
          children: [
            _ArticleFavoritesTab(),
            _PostFavoritesTab(),
            _BoardFavoritesTab(),
          ],
        ),
      ),
    );
  }
}

class _BoardFavoritesTab extends StatefulWidget {
  const _BoardFavoritesTab();

  @override
  State<_BoardFavoritesTab> createState() => _BoardFavoritesTabState();
}

class _BoardFavoritesTabState extends State<_BoardFavoritesTab> {
  Future<Result<Favorite>>? _future;

  @override
  void initState() {
    super.initState();
    _load();
  }

  void _load() {
    final future = AppScope.of(context).api.favorite.getFavorite('0');
    setState(() {
      _future = future;
    });
  }

  @override
  Widget build(BuildContext context) {
    return FutureBuilder<Result<Favorite>>(
      future: _future,
      builder: (context, snapshot) {
        if (snapshot.connectionState != ConnectionState.done) {
          return const LoadingView();
        }
        return ResultView<Favorite>(
          result: snapshot.data,
          onRetry: _load,
          builder: (data) {
            final boards = data.boards;
            final subs = data.subFavorites;
            if (boards.isEmpty && subs.isEmpty) {
              return const EmptyView(message: '还没有收藏版面');
            }
            return ListView(
              children: [
                for (final board in boards) _boardTile(board),
                for (final sub in subs)
                  ExpansionTile(
                    title: Text(
                      sub.section?.description?.isNotEmpty == true
                          ? sub.section!.description!
                          : (sub.section?.name ?? '收藏分组'),
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
    );
  }

  Widget _boardTile(Board board) {
    final name = board.name;
    if (name == null) return const SizedBox.shrink();
    return ListTile(
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

class _ArticleFavoritesTab extends StatefulWidget {
  const _ArticleFavoritesTab();

  @override
  State<_ArticleFavoritesTab> createState() => _ArticleFavoritesTabState();
}

class _ArticleFavoritesTabState extends State<_ArticleFavoritesTab> {
  Future<Result<Collection>>? _future;
  int _page = 1;

  @override
  void initState() {
    super.initState();
    _load();
  }

  void _load() {
    final future = AppScope.of(context).api.forum
        .getCollection(page: _page, count: 50);
    setState(() {
      _future = future;
    });
  }

  Future<void> _remove(CollectionArticle entry) async {
    final board = entry.bname;
    final id = entry.gid;
    if (board == null || id == null) return;
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (context) => AlertDialog(
        title: const Text('取消收藏'),
        content: Text('确定从收藏夹移除「${entry.title ?? '无标题'}」？'),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(context).pop(false),
            child: const Text('取消'),
          ),
          FilledButton(
            onPressed: () => Navigator.of(context).pop(true),
            child: const Text('移除'),
          ),
        ],
      ),
    );
    if (confirmed != true || !mounted) return;
    final result = await AppScope.of(context).api.forum
        .deleteCollection(board: board, id: '$id');
    if (!mounted) return;
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(
        content: Text(result.isSuccess ? '已取消收藏' : (result.message ?? '操作失败')),
      ),
    );
    if (result.isSuccess) _load();
  }

  @override
  Widget build(BuildContext context) {
    return FutureBuilder<Result<Collection>>(
      future: _future,
      builder: (context, snapshot) {
        if (snapshot.connectionState != ConnectionState.done) {
          return const LoadingView();
        }
        return ResultView<Collection>(
          result: snapshot.data,
          onRetry: _load,
          builder: (data) {
            if (data.entries.isEmpty) {
              return const EmptyView(message: '还没有收藏文章');
            }
            return Column(
              children: [
                Expanded(
                  child: ListView.separated(
                    itemCount: data.entries.length,
                    separatorBuilder: (_, _) => const Divider(height: 1),
                    itemBuilder: (context, index) {
                      final entry = data.entries[index];
                      final board = entry.bname;
                      final id = entry.gid;
                      return ListTile(
                        leading: UserAvatar(
                          user: entry.user,
                          radius: 18,
                          onTap: () => openUserProfile(context, entry.user),
                        ),
                        title: Text(
                          entry.title ?? '无标题',
                          maxLines: 2,
                          overflow: TextOverflow.ellipsis,
                        ),
                        subtitle: Text(
                          '${board ?? ''} · '
                          '${displayUserName(entry.user, fallback: '')} · '
                          '${entry.num ?? 0} 回复',
                          maxLines: 1,
                          overflow: TextOverflow.ellipsis,
                        ),
                        trailing: IconButton(
                          tooltip: '取消收藏',
                          onPressed: () => _remove(entry),
                          icon: const Icon(Icons.star, color: Colors.amber),
                        ),
                        onTap: board == null || id == null
                            ? null
                            : () => Navigator.of(context).push(
                                MaterialPageRoute(
                                  builder: (_) => ThreadScreen(
                                    boardName: board,
                                    threadId: '$id',
                                  ),
                                ),
                              ),
                      );
                    },
                  ),
                ),
                _Pager(
                  page: _page,
                  total: data.pagination?.pageAllCount,
                  onChanged: (delta) {
                    setState(() => _page += delta);
                    _load();
                  },
                ),
              ],
            );
          },
        );
      },
    );
  }
}

class _PostFavoritesTab extends StatefulWidget {
  const _PostFavoritesTab();

  @override
  State<_PostFavoritesTab> createState() => _PostFavoritesTabState();
}

class _PostFavoritesTabState extends State<_PostFavoritesTab> {
  Future<Result<FavorPost>>? _future;
  int _page = 1;

  @override
  void initState() {
    super.initState();
    _load();
  }

  void _load() {
    final future = AppScope.of(context).api.forum.getFavorPost(page: _page);
    setState(() {
      _future = future;
    });
  }

  @override
  Widget build(BuildContext context) {
    return FutureBuilder<Result<FavorPost>>(
      future: _future,
      builder: (context, snapshot) {
        if (snapshot.connectionState != ConnectionState.done) {
          return const LoadingView();
        }
        return ResultView<FavorPost>(
          result: snapshot.data,
          onRetry: _load,
          builder: (data) {
            if (data.articles.isEmpty) {
              return const EmptyView(message: '还没有收藏帖子');
            }
            return Column(
              children: [
                Expanded(
                  child: ListView.separated(
                    itemCount: data.articles.length,
                    separatorBuilder: (_, _) => const Divider(height: 1),
                    itemBuilder: (context, index) {
                      final article = data.articles[index];
                      final board = article.boardName;
                      final id = article.groupId ?? article.id;
                      return ListTile(
                        leading: UserAvatar(
                          user: article.user,
                          radius: 18,
                          onTap: () => openUserProfile(context, article.user),
                        ),
                        title: Text(
                          article.title ?? '无标题',
                          maxLines: 2,
                          overflow: TextOverflow.ellipsis,
                        ),
                        subtitle: Text(
                          '${board ?? ''} · '
                          '${displayUserName(article.user, fallback: '')} · '
                          '${article.replyCount ?? 0} 回复',
                          maxLines: 1,
                          overflow: TextOverflow.ellipsis,
                        ),
                        onTap: board == null || id == null
                            ? null
                            : () => Navigator.of(context).push(
                                MaterialPageRoute(
                                  builder: (_) => ThreadScreen(
                                    boardName: board,
                                    threadId: id,
                                  ),
                                ),
                              ),
                      );
                    },
                  ),
                ),
                _Pager(
                  page: _page,
                  total: data.pagination?.pageAllCount,
                  onChanged: (delta) {
                    setState(() => _page += delta);
                    _load();
                  },
                ),
              ],
            );
          },
        );
      },
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
  final ValueChanged<int> onChanged;

  @override
  Widget build(BuildContext context) {
    final totalPages = total;
    return SafeArea(
      top: false,
      child: Row(
        mainAxisAlignment: MainAxisAlignment.spaceBetween,
        children: [
          TextButton.icon(
            onPressed: page > 1 ? () => onChanged(-1) : null,
            icon: const Icon(Icons.chevron_left),
            label: const Text('上一页'),
          ),
          Text(
            totalPages == null || totalPages == 0
                ? '$page'
                : '$page / $totalPages',
          ),
          TextButton.icon(
            onPressed: totalPages == null || page >= totalPages
                ? null
                : () => onChanged(1),
            icon: const Icon(Icons.chevron_right),
            label: const Text('下一页'),
          ),
        ],
      ),
    );
  }
}

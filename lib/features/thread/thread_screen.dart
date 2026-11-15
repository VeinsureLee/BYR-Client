import 'package:material_ui/material_ui.dart';

import 'package:byr_client/app/app_scope.dart';
import 'package:byr_client/shared/storage/history_store.dart';
import 'package:byr_client/shared/core/result.dart';
import 'package:byr_client/shared/core/time_format.dart';
import 'package:byr_client/shared/data/models/article.dart';
import 'package:byr_client/shared/data/models/thread.dart';
import 'package:byr_client/shared/data/models/user.dart';
import 'package:byr_client/shared/widgets/common.dart';
import 'package:byr_client/shared/widgets/post_content.dart';
import 'package:byr_client/shared/widgets/reaction_button.dart';
import 'package:byr_client/shared/widgets/user_avatar.dart';
import 'package:byr_client/features/bet/bet_screen.dart';
import 'package:byr_client/features/board/board_screen.dart';
import 'package:byr_client/features/compose/compose_screen.dart';
import 'package:byr_client/features/vote/vote_screen.dart';

class ThreadScreen extends StatefulWidget {
  const ThreadScreen({
    super.key,
    required this.boardName,
    required this.threadId,
    this.fromBoard = false,
  });

  final String boardName;
  final String threadId;

  /// True when the previous route is the board list. Tapping the board badge
  /// then returns to that list instead of stacking another BoardScreen.
  final bool fromBoard;

  @override
  State<ThreadScreen> createState() => _ThreadScreenState();
}

class _ThreadScreenState extends State<ThreadScreen> {
  Future<Result<Thread>>? _future;
  Thread? _thread;
  int _page = 1;
  int? _totalPages;
  bool _collected = false;
  bool _collecting = false;

  @override
  void initState() {
    super.initState();
    _load();
  }

  void _load() {
    final history = AppScope.of(context).history;
    final future = AppScope.of(context).api.forum
        .getThread(board: widget.boardName, id: widget.threadId, page: _page);
    future.then((result) {
      if (!mounted) return;
      final thread = result.data;
      if (result.isSuccess && thread != null) {
        history.add(
          HistoryEntry(
            board: widget.boardName,
            id: widget.threadId,
            title: thread.title ?? '',
            visitedAt: DateTime.now().millisecondsSinceEpoch,
          ),
        );
        final current = thread.pagination?.pageCurrentCount;
        final total = thread.pagination?.pageAllCount;
        setState(() {
          _thread = thread;
          _totalPages = total;
          if (current != null && current > 0 && current != _page) {
            _page = current;
          }
        });
      }
    });
    setState(() {
      _future = future;
    });
  }

  void _changePage(int delta) {
    final next = _page + delta;
    if (next < 1) return;
    final total = _totalPages;
    if (total != null && total > 0 && next > total) return;
    setState(() => _page = next);
    _load();
  }

  Future<void> _like() async {
    final result = await AppScope.of(context).api.forum
        .likeArticle(board: widget.boardName, id: widget.threadId);
    if (!mounted) return;
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(
        content: Text(
          result.isSuccess
              ? '已赞（${result.data?.likeSum ?? 0}）'
              : (result.message ?? '操作失败'),
        ),
      ),
    );
  }

  Future<void> _collect() async {
    if (_collecting) return;
    final wasCollected = _collected;
    setState(() => _collecting = true);
    final api = AppScope.of(context).api;
    final result = wasCollected
        ? await api.forum.deleteCollection(
            board: widget.boardName,
            id: widget.threadId,
          )
        : await api.forum.addCollection(
            board: widget.boardName,
            id: widget.threadId,
          );
    if (!mounted) return;
    final message = result.message ?? '';
    setState(() {
      _collecting = false;
      if (result.isSuccess) {
        _collected = !wasCollected;
      } else if (message.contains('已经收录') || message.contains('已收藏')) {
        // The thread was already in the collection even though this page
        // started with `_collected = false`.
        _collected = true;
      }
    });
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(
        content: Text(
          result.isSuccess
              ? (wasCollected ? '已取消收藏' : '已加入收藏')
              : (message.contains('已经收录')
                    ? '已收藏（再次点击可取消）'
                    : (message.isEmpty ? '操作失败' : message)),
        ),
      ),
    );
  }

  Future<void> _share() async {
    final url =
        'https://bbs.byr.cn/article/${widget.boardName}/${widget.threadId}';
    await AppScope.of(context).device.copyText(url);
    if (!mounted) return;
    ScaffoldMessenger.of(context)
        .showSnackBar(const SnackBar(content: Text('帖子链接已复制')));
  }

  void _openBoard() {
    if (widget.fromBoard) {
      Navigator.of(context).pop();
      return;
    }
    Navigator.of(context).push(
      MaterialPageRoute(
        builder: (_) => BoardScreen(boardName: widget.boardName),
      ),
    );
  }

  Future<void> _replyThread() async {
    final posted = await Navigator.of(context).push<bool>(
      MaterialPageRoute(
        builder: (_) =>
            ComposeScreen(boardName: widget.boardName, reid: widget.threadId),
      ),
    );
    if (posted == true) _load();
  }

  Future<void> _edit() async {
    final api = AppScope.of(context).api;
    final result = await api.forum.getArticle(
      board: widget.boardName,
      id: widget.threadId,
    );
    if (!mounted) return;
    if (result.isFailure || result.data == null) {
      ScaffoldMessenger.of(context)
          .showSnackBar(SnackBar(content: Text(result.message ?? '无法加载帖子')));
      return;
    }
    final article = result.data!;
    final edited = await Navigator.of(context).push<bool>(
      MaterialPageRoute(
        builder: (_) => ComposeScreen(
          boardName: widget.boardName,
          editArticleId: widget.threadId,
          initialTitle: article.title,
          initialContent: plainText(article.content),
          initialAttachmentCount: article.attachments.fold(
            0,
            (count, attachment) => count + attachment.files.length,
          ),
          initialAttachments: article.attachments,
        ),
      ),
    );
    if (edited == true) _load();
  }

  Future<void> _delete() async {
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (context) => AlertDialog(
        title: const Text('删除帖子'),
        content: const Text('确定删除本帖？此操作不可撤销。'),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(context).pop(false),
            child: const Text('取消'),
          ),
          FilledButton(
            onPressed: () => Navigator.of(context).pop(true),
            child: const Text('删除'),
          ),
        ],
      ),
    );
    if (confirmed != true || !mounted) return;

    final result = await AppScope.of(context).api.forum
        .deleteArticle(board: widget.boardName, id: widget.threadId);
    if (!mounted) return;
    if (result.isFailure) {
      ScaffoldMessenger.of(context)
          .showSnackBar(SnackBar(content: Text(result.message ?? '删除失败')));
      return;
    }
    Navigator.of(context).pop(true);
  }

  bool get _canManageThread {
    final thread = _thread;
    if (thread == null) return false;
    if (thread.isAdmin == true) return true;
    final me = AppScope.of(context).api.session.username;
    if (me == null || me.isEmpty) return false;
    final author =
        thread.user?.id ??
        (thread.articles.isNotEmpty ? thread.articles.first.user?.id : null);
    return author == me;
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: const Text('帖子'),
        actions: [
          IconButton(
            onPressed: _like,
            icon: const Icon(Icons.thumb_up_alt_outlined),
            tooltip: '赞',
          ),
          IconButton(
            onPressed: _collecting ? null : _collect,
            icon: AnimatedSwitcher(
              duration: const Duration(milliseconds: 220),
              switchInCurve: Curves.easeOutBack,
              transitionBuilder: (child, animation) =>
                  ScaleTransition(scale: animation, child: child),
              child: Icon(
                _collected ? Icons.star : Icons.star_border,
                key: ValueKey<bool>(_collected),
                color: _collected ? Colors.amber : null,
              ),
            ),
            tooltip: _collected ? '取消收藏' : '收藏',
          ),
          IconButton(
            onPressed: _replyThread,
            icon: const Icon(Icons.reply),
            tooltip: '回复',
          ),
          PopupMenuButton<String>(
            onSelected: (value) {
              switch (value) {
                case 'vote':
                  Navigator.of(context).push(
                    MaterialPageRoute(
                      builder: (_) => VoteScreen(aid: widget.threadId),
                    ),
                  );
                  break;
                case 'bet':
                  Navigator.of(context).push(
                    MaterialPageRoute(
                      builder: (_) => BetScreen(bid: widget.threadId),
                    ),
                  );
                  break;
                case 'share':
                  _share();
                  break;
                case 'refresh':
                  _load();
                  break;
                case 'edit':
                  _edit();
                  break;
                case 'delete':
                  _delete();
                  break;
              }
            },
            itemBuilder: (_) => [
              const PopupMenuItem(value: 'vote', child: Text('投票')),
              const PopupMenuItem(value: 'bet', child: Text('竞猜')),
              const PopupMenuItem(value: 'share', child: Text('复制链接')),
              const PopupMenuItem(value: 'refresh', child: Text('刷新')),
              if (_canManageThread) ...[
                const PopupMenuItem(value: 'edit', child: Text('编辑本帖')),
                const PopupMenuItem(value: 'delete', child: Text('删除本帖')),
              ],
            ],
          ),
        ],
      ),
      body: FutureBuilder<Result<Thread>>(
        future: _future,
        builder: (context, snapshot) {
          if (snapshot.connectionState != ConnectionState.done) {
            return const LoadingView();
          }
          return ResultView<Thread>(
            result: snapshot.data,
            onRetry: _load,
            builder: (thread) {
              final posts = thread.articles;
              return Column(
                children: [
                  Expanded(
                    child: RefreshIndicator(
                      onRefresh: () async => _load(),
                      child: ListView(
                        padding: const EdgeInsets.all(12),
                        children: [
                          Align(
                            alignment: Alignment.centerLeft,
                            child: ActionChip(
                              avatar: const Icon(
                                Icons.forum_outlined,
                                size: 16,
                              ),
                              label: Text(
                                thread.boardDescription?.isNotEmpty == true
                                    ? '${thread.boardDescription} · ${thread.boardName ?? widget.boardName}'
                                    : (thread.boardName ?? widget.boardName),
                                maxLines: 1,
                                overflow: TextOverflow.ellipsis,
                              ),
                              onPressed: _openBoard,
                            ),
                          ),
                          const SizedBox(height: 8),
                          Text(
                            thread.title ??
                                (posts.isNotEmpty ? posts.first.title : null) ??
                                '无标题',
                            style: Theme.of(context).textTheme.titleLarge,
                          ),
                          const SizedBox(height: 12),
                          if (posts.isEmpty)
                            const EmptyView(message: '暂无帖子内容')
                          else
                            for (final post in posts)
                              _PostCard(post: post, onChanged: _load),
                        ],
                      ),
                    ),
                  ),
                  SafeArea(
                    top: false,
                    child: Row(
                      mainAxisAlignment: MainAxisAlignment.spaceBetween,
                      children: [
                        TextButton.icon(
                          onPressed: _page > 1 ? () => _changePage(-1) : null,
                          icon: const Icon(Icons.chevron_left),
                          label: const Text('上一页'),
                        ),
                        Text(
                          _totalPages == null || _totalPages! <= 0
                              ? '第 $_page 页'
                              : '第 $_page / $_totalPages 页',
                        ),
                        TextButton.icon(
                          onPressed:
                              _totalPages == null ||
                                  _totalPages! <= 0 ||
                                  _page < _totalPages!
                              ? () => _changePage(1)
                              : null,
                          icon: const Icon(Icons.chevron_right),
                          label: const Text('下一页'),
                        ),
                      ],
                    ),
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

class _PostCard extends StatefulWidget {
  const _PostCard({required this.post, this.onChanged});

  final Article post;
  final VoidCallback? onChanged;

  @override
  State<_PostCard> createState() => _PostCardState();
}

class _PostCardState extends State<_PostCard> {
  late int _likeSum = widget.post.likeSum ?? 0;
  late bool _liked = widget.post.isLiked ?? false;
  bool _liking = false;

  late int _voteDownSum = widget.post.votedownSum ?? 0;
  late bool _votedDown = widget.post.isVotedown ?? false;
  bool _votingDown = false;

  void _showMessage(String text) {
    ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(text)));
  }

  Future<void> _like() async {
    final post = widget.post;
    final board = post.boardName;
    final id = post.id;
    if (board == null || id == null || _liking) return;
    if (_liked) {
      _showMessage('已赞过，服务器不支持取消赞');
      return;
    }
    setState(() => _liking = true);
    final result = await AppScope.of(context).api.forum
        .likeArticle(board: board, id: id);
    if (!mounted) return;
    setState(() {
      _liking = false;
      if (result.isSuccess) {
        _liked = true;
        _votedDown = false;
        _likeSum = result.data?.likeSum ?? _likeSum + 1;
        _voteDownSum = result.data?.votedownSum ?? _voteDownSum;
      } else if ((result.message ?? '').contains('已赞过')) {
        _liked = true;
      }
    });
    _showMessage(
      result.isSuccess ? '已赞（$_likeSum）' : (result.message ?? '点赞失败'),
    );
  }

  Future<void> _voteDown() async {
    final post = widget.post;
    final board = post.boardName;
    final id = post.id;
    if (board == null || id == null || _votingDown) return;
    if (_votedDown) {
      _showMessage('已踩过，服务器不支持取消踩');
      return;
    }
    setState(() => _votingDown = true);
    final result = await AppScope.of(context).api.forum
        .voteDownArticle(board: board, id: id);
    if (!mounted) return;
    setState(() {
      _votingDown = false;
      if (result.isSuccess) {
        _votedDown = true;
        _liked = false;
        _voteDownSum = result.data?.votedownSum ?? _voteDownSum + 1;
        _likeSum = result.data?.likeSum ?? _likeSum;
      } else if ((result.message ?? '').contains('已踩过')) {
        _votedDown = true;
      }
    });
    _showMessage(
      result.isSuccess ? '已踩（$_voteDownSum）' : (result.message ?? '操作失败'),
    );
  }

  Future<void> _reply() async {
    final post = widget.post;
    final board = post.boardName;
    final id = post.id;
    if (board == null || id == null) return;
    final posted = await Navigator.of(context).push<bool>(
      MaterialPageRoute(
        builder: (_) => ComposeScreen(boardName: board, reid: id),
      ),
    );
    if (posted == true) widget.onChanged?.call();
  }

  @override
  Widget build(BuildContext context) {
    final post = widget.post;
    final postTime = formatTimestamp(post.postTime);
    return Card(
      margin: const EdgeInsets.only(bottom: 10),
      child: Padding(
        padding: const EdgeInsets.all(12),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                UserAvatar(
                  user: post.user,
                  radius: 18,
                  onTap: () => openUserProfile(context, post.user),
                ),
                const SizedBox(width: 8),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      GestureDetector(
                        onTap: () => openUserProfile(context, post.user),
                        child: Text(
                          displayUserName(post.user),
                          style: Theme.of(context).textTheme.titleSmall
                              ?.copyWith(fontWeight: FontWeight.bold),
                        ),
                      ),
                      if (postTime.isNotEmpty)
                        Text(
                          postTime,
                          style: Theme.of(context).textTheme.bodySmall,
                        ),
                    ],
                  ),
                ),
                if (post.position != null)
                  Text(
                    '#${post.position}',
                    style: Theme.of(context).textTheme.bodySmall,
                  ),
              ],
            ),
            const SizedBox(height: 8),
            PostContent(
              content: post.content,
              attachments: post.attachments,
              token: AppScope.of(context).api.client.token,
            ),
            const SizedBox(height: 4),
            Wrap(
              spacing: 4,
              children: [
                ReactionButton(
                  selected: _liked,
                  icon: Icons.thumb_up_alt_outlined,
                  selectedIcon: Icons.thumb_up,
                  label: '赞 $_likeSum',
                  busy: _liking,
                  onPressed: _like,
                ),
                ReactionButton(
                  selected: _votedDown,
                  icon: Icons.thumb_down_alt_outlined,
                  selectedIcon: Icons.thumb_down,
                  label: '踩 $_voteDownSum',
                  busy: _votingDown,
                  activeColor: Theme.of(context).colorScheme.error,
                  onPressed: _voteDown,
                ),
                TextButton.icon(
                  onPressed: _reply,
                  icon: const Icon(Icons.reply, size: 16),
                  label: const Text('回复'),
                ),
              ],
            ),
          ],
        ),
      ),
    );
  }
}

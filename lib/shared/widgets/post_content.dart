import 'package:cached_network_image/cached_network_image.dart';
import 'package:material_ui/material_ui.dart';
import 'package:byr_client/app/app_scope.dart';

import 'package:byr_client/shared/data/models/attachment.dart';
import 'package:byr_client/shared/widgets/common.dart';

/// Renders a post body, expanding nForum `[upload=N][/upload]` markers and
/// `[img]...[/img]` tags into real images.
///
/// nForum stores uploaded pictures as attachments and leaves an
/// `[upload=1][/upload]` placeholder in the content. The plain-text converter
/// used before this widget simply dropped those markers, which is why pictures
/// ("沙雕图") never appeared on any platform.
class PostContent extends StatelessWidget {
  const PostContent({
    super.key,
    required this.content,
    this.attachments = const [],
    this.token,
    this.maxImageWidth = 560,
  });

  final String? content;
  final List<Attachment> attachments;
  final String? token;
  final double maxImageWidth;

  List<FileElement> get _files => [
    for (final attachment in attachments) ...attachment.files,
  ];

  @override
  Widget build(BuildContext context) {
    final files = _files;
    final used = <int>{};
    final widgets = _parseBody(context, files, used);

    final leftoverImages = <FileElement>[];
    final leftoverFiles = <FileElement>[];
    for (var i = 0; i < files.length; i++) {
      if (used.contains(i)) continue;
      if (_isImage(files[i])) {
        leftoverImages.add(files[i]);
      } else {
        leftoverFiles.add(files[i]);
      }
    }

    if (leftoverImages.isNotEmpty) {
      widgets.add(
        Padding(
          padding: const EdgeInsets.only(top: 8),
          child: Wrap(
            spacing: 8,
            runSpacing: 8,
            children: [
              for (final file in leftoverImages)
                _Thumbnail(file: file, token: token, maxSize: 140),
            ],
          ),
        ),
      );
    }
    for (final file in leftoverFiles) {
      widgets.add(
        Padding(
          padding: const EdgeInsets.only(top: 4),
          child: _FileTile(file: file, token: token),
        ),
      );
    }

    if (widgets.isEmpty) {
      return const SelectableText('(无内容)');
    }
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: widgets,
    );
  }

  List<Widget> _parseBody(
    BuildContext context,
    List<FileElement> files,
    Set<int> used,
  ) {
    final text = (content ?? '').replaceAllMapped(
      RegExp(
        r'''<img[^>]*src\s*=\s*["']([^"']+)["'][^>]*>''',
        caseSensitive: false,
      ),
      (match) => '[img]${match.group(1)}[/img]',
    );

    final pattern = RegExp(
      r'\[upload=(\d+)\](?:\s*\[/upload\])?|\[img(?:=[^\]]*)?\](.*?)\[/img\]',
      caseSensitive: false,
      dotAll: true,
    );

    final widgets = <Widget>[];
    var cursor = 0;

    void addText(String raw) {
      // Drop closing markers left over when the opening tag and closing tag
      // are separated by whitespace.
      final cleaned = raw.replaceAll(
        RegExp(r'\[/upload\]', caseSensitive: false),
        '',
      );
      final spans = _buildTextSpans(context, cleaned);
      if (spans.isEmpty) return;
      widgets.add(
        SelectableText.rich(
          TextSpan(children: spans),
          style: Theme.of(context).textTheme.bodyMedium,
        ),
      );
    }

    for (final match in pattern.allMatches(text)) {
      addText(text.substring(cursor, match.start));
      cursor = match.end;

      final uploadIndex = int.tryParse(match.group(1) ?? '');
      if (uploadIndex != null) {
        final index = uploadIndex - 1;
        if (index < 0 || index >= files.length) {
          widgets.add(const _BrokenMarker(text: '[图片缺失]'));
          continue;
        }
        final file = files[index];
        used.add(index);
        if (_isImage(file)) {
          final url = _prepareUrl(_fileThumbUrl(file));
          if (url == null) {
            widgets.add(const _BrokenMarker(text: '[图片地址为空]'));
          } else {
            widgets.add(
              _PostImage(
                url: url,
                fullUrl: _prepareUrl(file.url) ?? url,
                name: file.name,
                maxWidth: maxImageWidth,
              ),
            );
          }
        } else {
          widgets.add(_FileTile(file: file, token: token));
        }
        continue;
      }

      final rawUrl = match.group(2);
      final url = _prepareUrl(rawUrl);
      if (url == null) {
        widgets.add(const _BrokenMarker(text: '[图片地址为空]'));
      } else {
        widgets.add(
          _PostImage(
            url: url,
            fullUrl: url,
            name: null,
            maxWidth: maxImageWidth,
          ),
        );
      }
    }
    addText(text.substring(cursor));
    return widgets;
  }

  /// Renders `[em1]`/`[emc50r]` markers as the original GIF emoji assets.
  List<InlineSpan> _buildTextSpans(BuildContext context, String raw) {
    final spans = <InlineSpan>[];
    final pattern = RegExp(r'\[em([a-zA-Z0-9]+)\]', caseSensitive: false);
    var cursor = 0;

    for (final match in pattern.allMatches(raw)) {
      final before = stripHtml(raw.substring(cursor, match.start));
      if (before.isNotEmpty) spans.add(TextSpan(text: before));
      final name = match.group(1)!;
      spans.add(
        WidgetSpan(
          alignment: PlaceholderAlignment.middle,
          child: Padding(
            padding: const EdgeInsets.symmetric(horizontal: 1),
            child: Image.asset(
              'assets/em/em$name.gif',
              width: 22,
              height: 22,
              errorBuilder: (context, error, stack) => Text(
                '[em$name]',
                style: Theme.of(context).textTheme.bodySmall,
              ),
            ),
          ),
        ),
      );
      cursor = match.end;
    }

    final tail = stripHtml(raw.substring(cursor));
    if (tail.isNotEmpty) spans.add(TextSpan(text: tail));
    return spans;
  }

  String? _fileThumbUrl(FileElement file) {
    final middle = file.thumbnailMiddle;
    if (middle != null && middle.isNotEmpty) return middle;
    final small = file.thumbnailSmall;
    if (small != null && small.isNotEmpty) return small;
    return file.url;
  }

  String? _prepareUrl(String? raw) => prepareAttachmentUrl(raw, token);

  static bool _isImage(FileElement file) {
    final thumb = file.thumbnailMiddle ?? file.thumbnailSmall;
    if (thumb != null && thumb.isNotEmpty) return true;
    final url = (file.url ?? '').toLowerCase();
    final path = Uri.tryParse(url)?.path ?? url;
    return const [
      '.jpg',
      '.jpeg',
      '.png',
      '.gif',
      '.webp',
      '.bmp',
      '.svg',
    ].any(path.endsWith);
  }
}

/// Resolves and authenticates an attachment/thumbnail URL.
///
/// nForum may return relative paths (`/api/attachment/...`) and the v4 app
/// rewrites the old `/api/` prefix to `/open/` before adding the token.
String? prepareAttachmentUrl(String? raw, String? token) {
  if (raw == null || raw.isEmpty) return null;
  var url = absoluteUrl(raw);
  if (url == null) return null;
  // v3 clients used `/api/...`; v4 serves the same files behind `/open/...`.
  if (url.contains('/api/')) {
    url = url.replaceFirst('/api/', '/open/');
  }
  return appendOAuthToken(url, token);
}

/// Adds `oauth_token` to BYR-hosted URLs (attachments are protected just like
/// the rest of the `/open` API).
String appendOAuthToken(String url, String? token) {
  if (token == null || token.isEmpty) return url;
  final uri = Uri.tryParse(url);
  if (uri == null) return url;
  if (uri.host.isNotEmpty && !uri.host.endsWith('byr.cn')) return url;
  if (uri.queryParameters.containsKey('oauth_token')) return url;
  return uri
      .replace(queryParameters: {...uri.queryParameters, 'oauth_token': token})
      .toString();
}

String _referer() => 'https://bbs.byr.cn/';

class _PostImage extends StatelessWidget {
  const _PostImage({
    required this.url,
    required this.fullUrl,
    required this.name,
    required this.maxWidth,
  });

  final String url;
  final String fullUrl;
  final String? name;
  final double maxWidth;

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 6),
      child: ConstrainedBox(
        constraints: BoxConstraints(maxWidth: maxWidth, maxHeight: 420),
        child: ClipRRect(
          borderRadius: BorderRadius.circular(8),
          child: CachedNetworkImage(
            imageUrl: url,
            httpHeaders: {'Referer': _referer()},
            fit: BoxFit.contain,
            placeholder: (context, _) => Container(
              height: 180,
              alignment: Alignment.center,
              color: Theme.of(context).colorScheme.surfaceContainerHighest,
              child: const SizedBox(
                width: 24,
                height: 24,
                child: CircularProgressIndicator(strokeWidth: 2),
              ),
            ),
            errorWidget: (context, failedUrl, error) => InkWell(
              onTap: () => _openViewer(context, fullUrl),
              child: _ImageError(name: name, detail: '$error'),
            ),
          ),
        ),
      ),
    );
  }
}

class _ImageError extends StatelessWidget {
  const _ImageError({this.name, this.detail});

  final String? name;
  final String? detail;

  @override
  Widget build(BuildContext context) {
    return Container(
      constraints: const BoxConstraints(minWidth: 180, minHeight: 100),
      padding: const EdgeInsets.all(12),
      color: Theme.of(context).colorScheme.surfaceContainerHighest,
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          const Icon(Icons.broken_image_outlined, size: 32),
          const SizedBox(height: 6),
          Text(
            name?.isNotEmpty == true ? name! : '图片加载失败',
            textAlign: TextAlign.center,
            style: Theme.of(context).textTheme.bodySmall,
          ),
          if (detail != null && detail!.isNotEmpty)
            Padding(
              padding: const EdgeInsets.only(top: 4),
              child: Text(
                detail!,
                maxLines: 2,
                overflow: TextOverflow.ellipsis,
                textAlign: TextAlign.center,
                style: Theme.of(context).textTheme.labelSmall,
              ),
            ),
          const SizedBox(height: 4),
          Text('点击查看原图', style: Theme.of(context).textTheme.labelSmall),
        ],
      ),
    );
  }
}

class _Thumbnail extends StatelessWidget {
  const _Thumbnail({
    required this.file,
    required this.token,
    required this.maxSize,
  });

  final FileElement file;
  final String? token;
  final double maxSize;

  @override
  Widget build(BuildContext context) {
    final raw = file.thumbnailMiddle?.isNotEmpty == true
        ? file.thumbnailMiddle
        : (file.thumbnailSmall?.isNotEmpty == true
              ? file.thumbnailSmall
              : file.url);
    final url = prepareAttachmentUrl(raw, token);
    if (url == null) return const SizedBox.shrink();
    final full = prepareAttachmentUrl(file.url, token) ?? url;
    return InkWell(
      onTap: () => _openViewer(context, full),
      child: ClipRRect(
        borderRadius: BorderRadius.circular(6),
        child: CachedNetworkImage(
          imageUrl: url,
          httpHeaders: {'Referer': _referer()},
          width: maxSize,
          height: maxSize,
          fit: BoxFit.cover,
          placeholder: (context, _) => Container(
            width: maxSize,
            height: maxSize,
            alignment: Alignment.center,
            color: Theme.of(context).colorScheme.surfaceContainerHighest,
            child: const SizedBox(
              width: 18,
              height: 18,
              child: CircularProgressIndicator(strokeWidth: 2),
            ),
          ),
          errorWidget: (context, failedUrl, error) => InkWell(
            onTap: () => _openViewer(context, full),
            child: SizedBox(
              width: maxSize,
              height: maxSize,
              child: _ImageError(name: file.name),
            ),
          ),
        ),
      ),
    );
  }
}

class _FileTile extends StatelessWidget {
  const _FileTile({required this.file, this.token});

  final FileElement file;
  final String? token;

  @override
  Widget build(BuildContext context) {
    final base = absoluteUrl(file.url);
    final url = base == null ? null : appendOAuthToken(base, token);
    return InkWell(
      onTap: url == null
          ? null
          : () => AppScope.of(context).device.openLink(Uri.parse(url)),
      child: Row(
        children: [
          const Icon(Icons.attach_file, size: 16),
          const SizedBox(width: 4),
          Expanded(
            child: Text(
              file.name ?? '附件',
              style: Theme.of(context).textTheme.bodySmall,
            ),
          ),
        ],
      ),
    );
  }
}

class _BrokenMarker extends StatelessWidget {
  const _BrokenMarker({required this.text});

  final String text;

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 4),
      child: Text(
        text,
        style: TextStyle(color: Theme.of(context).colorScheme.error),
      ),
    );
  }
}

void _openViewer(BuildContext context, String url) {
  showDialog<void>(
    context: context,
    builder: (context) => Dialog(
      insetPadding: const EdgeInsets.all(24),
      child: ConstrainedBox(
        constraints: BoxConstraints(
          maxWidth: 1200,
          maxHeight: MediaQuery.sizeOf(context).height * 0.85,
        ),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Padding(
              padding: const EdgeInsets.fromLTRB(16, 8, 8, 0),
              child: Row(
                children: [
                  const Expanded(child: Text('图片')),
                  IconButton(
                    tooltip: '在浏览器中打开',
                    onPressed: () =>
                        AppScope.of(context).device.openLink(Uri.parse(url)),
                    icon: const Icon(Icons.open_in_new),
                  ),
                  IconButton(
                    tooltip: '关闭',
                    onPressed: () => Navigator.of(context).pop(),
                    icon: const Icon(Icons.close),
                  ),
                ],
              ),
            ),
            Flexible(
              child: InteractiveViewer(
                child: Image.network(
                  url,
                  headers: {'Referer': _referer()},
                  fit: BoxFit.contain,
                  loadingBuilder: (context, child, progress) {
                    if (progress == null) return child;
                    return const Padding(
                      padding: EdgeInsets.all(32),
                      child: CircularProgressIndicator(),
                    );
                  },
                  errorBuilder: (context, error, stack) => Padding(
                    padding: const EdgeInsets.all(24),
                    child: _ImageError(name: '图片加载失败', detail: '$error'),
                  ),
                ),
              ),
            ),
            const SizedBox(height: 8),
          ],
        ),
      ),
    ),
  );
}

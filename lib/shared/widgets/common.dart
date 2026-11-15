import 'package:material_ui/material_ui.dart';

import 'package:byr_client/shared/core/constants.dart';
import 'package:byr_client/shared/core/result.dart';

class LoadingView extends StatelessWidget {
  const LoadingView({super.key});

  @override
  Widget build(BuildContext context) =>
      const Center(child: CircularProgressIndicator());
}

class ErrorView extends StatelessWidget {
  const ErrorView({super.key, required this.message, this.onRetry});

  final String message;
  final VoidCallback? onRetry;

  @override
  Widget build(BuildContext context) {
    return Center(
      child: Padding(
        padding: const EdgeInsets.all(24),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Text(
              message,
              textAlign: TextAlign.center,
              style: TextStyle(color: Theme.of(context).colorScheme.error),
            ),
            if (onRetry != null) ...[
              const SizedBox(height: 12),
              OutlinedButton(onPressed: onRetry, child: const Text('重试')),
            ],
          ],
        ),
      ),
    );
  }
}

class EmptyView extends StatelessWidget {
  const EmptyView({super.key, this.message = '暂无内容'});

  final String message;

  @override
  Widget build(BuildContext context) => Center(child: Text(message));
}

/// Renders a value returned by the API layer.
class ResultView<T> extends StatelessWidget {
  const ResultView({
    super.key,
    required this.result,
    required this.builder,
    this.onRetry,
  });

  final Result<T>? result;
  final Widget Function(T data) builder;
  final VoidCallback? onRetry;

  @override
  Widget build(BuildContext context) {
    final value = result;
    if (value == null || value.state == ResultState.loading) {
      return const LoadingView();
    }
    if (value.isFailure || value.data == null) {
      return ErrorView(message: value.message ?? '加载失败', onRetry: onRetry);
    }
    return builder(value.data as T);
  }
}

/// Resolves relative media paths (avatars, attachments) against the BYR host.
String? absoluteUrl(String? url) {
  if (url == null || url.isEmpty) return null;
  if (url.startsWith('http://') || url.startsWith('https://')) return url;
  if (url.startsWith('//')) return 'https:$url';
  return '${Constants.host}$url';
}

/// Small UBB/HTML to text converter. Does not trim, so it can be used on
/// fragments inside a rich-text paragraph.
String stripHtml(String? input) {
  if (input == null || input.isEmpty) return '';
  return input
      .replaceAll(RegExp(r'<br\s*/?>', caseSensitive: false), '\n')
      .replaceAll(RegExp(r'<[^>]+>'), '')
      .replaceAll('&nbsp;', ' ')
      .replaceAll('&lt;', '<')
      .replaceAll('&gt;', '>')
      .replaceAll('&amp;', '&')
      .replaceAll('&quot;', '"');
}

/// Very small UBB/HTML to plain text converter for list previews.
String plainText(String? input) => stripHtml(input).trim();

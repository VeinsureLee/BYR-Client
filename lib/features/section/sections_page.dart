import 'package:material_ui/material_ui.dart';
import 'package:byr_client/app/app_scope.dart';
import 'package:byr_client/shared/core/result.dart';
import 'package:byr_client/shared/data/models/root_section.dart';
import 'package:byr_client/shared/widgets/common.dart';
import 'package:byr_client/features/search/search_screen.dart';
import 'package:byr_client/features/section/section_screen.dart';

class SectionsPage extends StatefulWidget {
  const SectionsPage({super.key});

  @override
  State<SectionsPage> createState() => _SectionsPageState();
}

class _SectionsPageState extends State<SectionsPage> {
  Future<Result<RootSection>>? _future;

  @override
  void initState() {
    super.initState();
    _load();
  }

  void _load() {
    final future = AppScope.of(context).api.section.getRootSection();
    setState(() {
      _future = future;
    });
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: const Text('首页'),
        actions: [
          IconButton(
            onPressed: () => Navigator.of(context)
                .push(MaterialPageRoute(builder: (_) => const SearchScreen())),
            icon: const Icon(Icons.search),
            tooltip: '搜索',
          ),
        ],
      ),
      body: FutureBuilder<Result<RootSection>>(
        future: _future,
        builder: (context, snapshot) {
          if (snapshot.connectionState != ConnectionState.done) {
            return const LoadingView();
          }
          return ResultView<RootSection>(
            result: snapshot.data,
            onRetry: _load,
            builder: (data) {
              if (data.sections.isEmpty) {
                return const EmptyView();
              }
              return ListView.separated(
                itemCount: data.sections.length,
                separatorBuilder: (_, _) => const Divider(height: 1),
                itemBuilder: (context, index) {
                  final section = data.sections[index];
                  return ListTile(
                    title: Text(
                      section.description?.isNotEmpty == true
                          ? section.description!
                          : (section.name ?? '未命名分区'),
                    ),
                    trailing: const Icon(Icons.chevron_right),
                    onTap: () {
                      if (section.name != null) {
                        Navigator.of(context).push(
                          MaterialPageRoute(
                            builder: (_) => SectionScreen(
                              sectionName: section.name!,
                              sectionTitle: section.description,
                            ),
                          ),
                        );
                      }
                    },
                  );
                },
              );
            },
          );
        },
      ),
    );
  }
}

import 'package:material_ui/material_ui.dart';

import 'package:byr_client/app/app_scope.dart';

/// Delegates shell selection to the application's injected layout Strategy.
class AppHome extends StatelessWidget {
  const AppHome({super.key});

  @override
  Widget build(BuildContext context) =>
      AppScope.of(context).homeBuilder(context);
}

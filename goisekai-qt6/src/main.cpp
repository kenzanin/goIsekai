#include <QGuiApplication>
#include <QQmlApplicationEngine>
#include <QQmlContext>
#include <QCommandLineOption>
#include <QCommandLineParser>

int main(int argc, char *argv[])
{
    QGuiApplication app(argc, argv);
    QCoreApplication::setApplicationName(QStringLiteral("goisekai-qt6"));

    QCommandLineParser parser;
    parser.addHelpOption();
    const QCommandLineOption serverOpt(
        {QStringLiteral("s"), QStringLiteral("server")},
        QStringLiteral("goisekai server base URL"),
        QStringLiteral("url"),
        QStringLiteral("http://127.0.0.1:8080"));
    parser.addOption(serverOpt);
    parser.process(app);

    QQmlApplicationEngine engine;
    engine.rootContext()->setContextProperty(QStringLiteral("serverBase"),
                                             parser.value(serverOpt));

    QObject::connect(
        &engine, &QQmlApplicationEngine::objectCreationFailed, &app,
        []() { QCoreApplication::exit(1); }, Qt::QueuedConnection);

    engine.load(QUrl(QStringLiteral("qrc:/qt/qml/Goisekai/qml/Main.qml")));
    if (engine.rootObjects().isEmpty())
        return 1;

    return app.exec();
}

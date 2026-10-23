cmake_minimum_required(VERSION 3.21)
project(goisekai-qt6 LANGUAGES CXX)

set(CMAKE_CXX_STANDARD 17)
set(CMAKE_CXX_STANDARD_REQUIRED ON)

find_package(Qt6 6.5 REQUIRED COMPONENTS Quick Gui)
qt_standard_project_setup(REQUIRES 6.5)

# qml/ is an extra directory inside the module, so it needs its own generated
# qmldir — QTP0004 NEW does that (the OLD behaviour silently loses the singleton
# registration, which is what made Theme.* undefined).
qt_policy(SET QTP0004 NEW)

qt_add_executable(goisekai-qt6 src/main.cpp)
target_link_libraries(goisekai-qt6 PRIVATE Qt6::Quick Qt6::Gui)

# Theme is a QML singleton (pragma Singleton); without this property the
# generated qmldir has no "singleton" line and every Theme.* reference fails.
set_source_files_properties(qml/Theme.qml PROPERTIES QT_QML_SINGLETON_TYPE TRUE)

qt_add_qml_module(goisekai-qt6
    URI Goisekai
    VERSION 1.0
    QML_FILES
        qml/Theme.qml
        qml/MangaCard.qml
        qml/Main.qml
)

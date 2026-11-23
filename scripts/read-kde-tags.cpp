#include <KFileMetaData/UserMetaData>
#include <QCoreApplication>
#include <QDebug>
int main(int argc,char **argv) {
    QCoreApplication app(argc,argv);
    const auto tags=KFileMetaData::UserMetaData(app.arguments().at(1)).tags();
    qInfo() << tags;
    return !(tags.contains("work") && tags.join(",").contains(" burial"));
}

from setuptools import setup


def read_file(path):
    with open(path) as file:
        return file.read()


setup(
    name="magneticod",
    version="0.4.0",
    description="Autonomous BitTorrent DHT crawler and metadata fetcher.",
    long_description=read_file("README.rst"),
    url="http://magnetico.org",
    author="Mert Bora ALPER",
    author_email="bora@boramalper.org",
    license="GNU Affero General Public License v3 or later (AGPLv3+)",
    packages=["magneticod"],
    zip_safe=False,
    entry_points={
        "console_scripts": ["magneticod=magneticod.__main__:main"]
    },

    install_requires=[
        "appdirs >= 1.4.3",
        "bencoder.pyx >= 1.1.3",
        "humanfriendly"
    ],

    classifiers=[
        "Development Status :: 2 - Pre-Alpha",
        "Environment :: No Input/Output (Daemon)",
        "Intended Audience :: End Users/Desktop",
        "License :: OSI Approved :: GNU Affero General Public License v3 or later (AGPLv3+)",
        "Natural Language :: English",
        "Operating System :: POSIX :: Linux",
        "Programming Language :: Python :: 3 :: Only",
        "Programming Language :: Python :: Implementation :: CPython",
    ]
)
